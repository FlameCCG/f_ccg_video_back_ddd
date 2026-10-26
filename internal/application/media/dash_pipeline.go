package media

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// dashPipelineRequest 是 DASH 管线的一次运行请求。
type dashPipelineRequest struct {
	task       domain.TranscodeTask
	policy     *domain.TranscodePolicy
	checkpoint Checkpoint
	srcPath    string
	driver     domain.StorageDriver
	meta       domain.VideoMeta
	plan       domain.PipelinePlan
	tracker    *domain.TranscodeProgressTracker
}

// runDASHPipeline 产出覆盖阶梯各清晰度的 DASH 自适应流，返回 manifest.mpd 的可访问 URL
// 与总字节数（原 runDASHPipeline）。
//
// GPU/CPU 网关：两者都开 → 先 GPU、不行再 CPU；只开其一 → 按开启的模式来。
func (s *TranscodeService) runDASHPipeline(ctx context.Context, req dashPipelineRequest) (string, int64, error) {
	if req.policy == nil || !req.policy.DashEnable || len(req.plan.DashResolutions) == 0 {
		return "", 0, nil
	}

	if err := ctx.Err(); err != nil {
		return "", 0, fmt.Errorf("dash pipeline canceled before bootstrap: %w", err)
	}

	useGPU := req.policy.GpuMode
	if useGPU {
		if until, active := s.gpuCooldown.Active(); active {
			useGPU = false

			logctx.With(ctx, s.logger).Warn("skip GPU dash during cooldown", zap.String("event", "media.dash_pipeline.run_dash_pipeline.degraded"), zap.String("operation", "media.dash_pipeline.run_dash_pipeline"), zap.Time("retry_after", until))
		}
	}

	if useGPU {
		mpdURL, size, err := s.runDASHAttempt(ctx, req, true)
		if err == nil {
			return mpdURL, size, nil
		}

		if ctx.Err() != nil {
			return mpdURL, size, err
		}

		s.noteGPUFailure(ctx, err)
		logctx.With(ctx, s.logger).Warn("dash GPU failed, fallback to CPU", zap.String("event", "media.dash_pipeline.run_dash_pipeline.degraded"), zap.String("operation", "media.dash_pipeline.run_dash_pipeline"), zap.Error(err))
	}

	return s.runDASHAttempt(ctx, req, false)
}

// runDASHAttempt 执行一次 DASH 编码尝试并上传整个产物目录。
func (s *TranscodeService) runDASHAttempt(
	ctx context.Context,
	req dashPipelineRequest,
	useGPU bool,
) (string, int64, error) {
	targets := domain.BuildDashTargets(
		req.plan.DashResolutions,
		req.plan.PixelHeight,
		req.plan.ClassHeight,
		req.plan.HighBitrate,
	)

	chunkPlan := req.policy.ChunkPlan(req.meta.Duration, req.checkpoint.Persistent())

	committedDir, err := s.encoder.EncodeDASHDir(ctx, DASHEncodeRequest{
		Task:           req.task,
		Policy:         req.policy,
		Checkpoint:     req.checkpoint,
		SourcePath:     req.srcPath,
		Targets:        targets,
		UseGPU:         useGPU,
		SourceBitrate:  req.meta.Bitrate,
		SourceDuration: req.meta.Duration,
		SourceCodec:    req.meta.Codec,
		HasAudio:       req.meta.HasAudio(),
		ChunkPlan:      chunkPlan,
	})
	if err != nil {
		return "", 0, err
	}

	req.tracker.MarkPipelineUploading(ctx, domain.TranscodePipelineDASH, "上传 DASH 资源")

	return s.uploadDASHDir(ctx, req.task, req.driver, committedDir)
}

// uploadDASHDir 遍历 DASH 产物目录，逐个流式上传到 dash 前缀下（原 uploadDASHDir）。
//
// 先上传各 representation 文件、最后上传 manifest，
// 保证 manifest 可达时其引用的 representation 均已存在。
func (s *TranscodeService) uploadDASHDir(
	ctx context.Context,
	task domain.TranscodeTask,
	driver domain.StorageDriver,
	dir string,
) (string, int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, fmt.Errorf("read dash temp dir failed: %w", err)
	}

	var total int64

	hasManifest := false

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == domain.DashManifestName {
			if entry.Name() == domain.DashManifestName {
				hasManifest = true
			}

			continue
		}

		_, size, err := s.uploadOneDASHFile(ctx, task, driver, dir, entry.Name())
		if err != nil {
			return "", 0, fmt.Errorf("upload dash file %q failed: %w", entry.Name(), err)
		}

		total += size
	}

	if !hasManifest {
		return "", 0, errors.New("dash manifest not produced")
	}

	mpdURL, size, err := s.uploadOneDASHFile(ctx, task, driver, dir, domain.DashManifestName)
	if err != nil {
		return "", 0, fmt.Errorf("upload dash manifest failed: %w", err)
	}

	total += size

	return mpdURL, total, nil
}

// uploadOneDASHFile 上传 DASH 目录中的单个文件（按体积估算超时）。
func (s *TranscodeService) uploadOneDASHFile(
	ctx context.Context,
	task domain.TranscodeTask,
	driver domain.StorageDriver,
	dir, name string,
) (string, int64, error) {
	f, err := os.Open(filepath.Join(dir, name)) //nolint:gosec // 路径来自本进程生成的断点目录产物
	if err != nil {
		return "", 0, err
	}

	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}

	rel := path.Join(task.DashPrefix(), name)
	label := fmt.Sprintf("video-%d-dash-%s", task.VideoID, name)

	uploadTimeout := domain.CalcUploadTimeout(fi.Size())

	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	url, size, err := driver.SaveTranscodeVideo(uploadCtx, rel, fi.Size(), f, DashContentType(name), label)
	if err != nil {
		if errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
			return "", 0, fmt.Errorf("%w after %s for %s (%d bytes): %w",
				domain.ErrTranscodeUploadTimeout, uploadTimeout, name, fi.Size(), err)
		}

		return "", 0, err
	}

	return url, size, nil
}

// DashContentType 为 DASH 产物推断 Content-Type（供对象存储落对象元数据用）。
func DashContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".mpd":
		return "application/dash+xml"
	case ".m4s":
		return "video/iso.segment"
	default:
		return "video/mp4"
	}
}
