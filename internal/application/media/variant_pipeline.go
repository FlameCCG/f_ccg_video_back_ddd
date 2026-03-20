package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	mediaruntime "f_ccg_video_back_ddd/internal/application/media/runtime"
	mediaerrors "f_ccg_video_back_ddd/internal/domain/media/domainerr"
	mediaentity "f_ccg_video_back_ddd/internal/domain/media/entity"
	mediaport "f_ccg_video_back_ddd/internal/domain/media/port"
	mediaservice "f_ccg_video_back_ddd/internal/domain/media/service"
	mediavo "f_ccg_video_back_ddd/internal/domain/media/valueobject"

	"go.uber.org/zap"
)

// uploadResult 是一路产物上传后的结果。
type uploadResult struct {
	targetID   string
	resolution uint16
	fileURL    string
	fileSize   int64
	bitrate    int
	fileName   string
}

// variantPipelineRequest 是 MP4 多输出管线的一次运行请求。
type variantPipelineRequest struct {
	task       mediaentity.TranscodeTask
	policy     *mediavo.TranscodePolicy
	checkpoint Checkpoint
	srcPath    string
	targets    []mediavo.TranscodeTarget
	driver     mediaport.StorageDriver
	meta       mediavo.VideoMeta
}

// executeVariantPipeline 运行 MP4 多输出编码并上传产物，GPU 失败自动回退 CPU。
//
// GPU 冷却期内直接跳过 GPU；GPU 失败时按错误关键字进入冷却，
// CPU 回退可经断点复用 GPU 已提交的完整产物（编码成功但上传失败时不再重编）。
func (s *TranscodeService) executeVariantPipeline(
	ctx context.Context,
	req variantPipelineRequest,
) (map[string]uploadResult, error) {
	useGPU := req.policy != nil && req.policy.GpuMode
	if useGPU {
		if until, active := s.gpuCooldown.Active(); active {
			useGPU = false

			s.logger.Warn("skip GPU pipeline during cooldown", zap.Time("retryAfter", until))
		}
	}

	if useGPU {
		results, err := s.runMP4Encode(ctx, req, true)
		if err == nil {
			return results, nil
		}

		if ctx.Err() != nil {
			return results, err
		}

		s.noteGPUFailure(err)
		// CPU 回退会经由断点复用 GPU 已提交的完整产物（编码成功但上传失败时不再重编）。
		// 分段块按编码模式隔离，CPU 不复用 GPU 块（参数集不同禁止混拼），但已拼装的最终产物可复用。
		s.logger.Warn("GPU pipeline failed, fallback to CPU", zap.Error(err))
	}

	// CPU 回退。最终失败时只清理独立的规范化源输出，原始输入永远不参与清理。
	results, err := s.runMP4Encode(ctx, req, false)
	if err != nil {
		s.cleanupFailedNormalizedTargets(ctx, req.srcPath, req.targets, req.driver)

		for _, target := range req.targets {
			if target.IsSource {
				delete(results, target.ID)
			}
		}
	}

	return results, err
}

// noteGPUFailure 按错误内容决定是否进入 GPU 冷却，并记录日志。
func (s *TranscodeService) noteGPUFailure(err error) {
	until, cooldown, entered := s.gpuCooldown.NoteFailure(err)
	if !entered {
		return
	}

	s.logger.Warn("disable GPU transcode temporarily",
		zap.Time("retryAfter", until),
		zap.Duration("cooldown", cooldown),
		zap.Error(err),
	)
}

// runMP4Encode 执行一次 MP4 编码尝试并上传产物。
//
// 断点复用（L2）、分段转码与产物提交在 infrastructure 的编码器适配器内完成；
// 本函数负责源/目标隔离校验、取消检查与串行上传。
func (s *TranscodeService) runMP4Encode(
	ctx context.Context,
	req variantPipelineRequest,
	useGPU bool,
) (map[string]uploadResult, error) {
	if len(req.targets) == 0 {
		return nil, errors.New("no transcode targets")
	}

	if err := mediaservice.ValidateSourceTargetIsolation(req.srcPath, req.targets); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("transcode pipeline canceled before bootstrap: %w", err)
	}

	chunkPlan := req.policy.ChunkPlan(req.meta.Duration, req.checkpoint.Persistent())

	localPaths, err := s.encoder.EncodeMP4Targets(ctx, MP4EncodeRequest{
		Task:           req.task,
		Policy:         req.policy,
		Checkpoint:     req.checkpoint,
		SourcePath:     req.srcPath,
		Targets:        req.targets,
		UseGPU:         useGPU,
		SourceBitrate:  req.meta.Bitrate,
		SourceDuration: req.meta.Duration,
		SourceCodec:    req.meta.Codec,
		HasAudio:       req.meta.HasAudio(),
		ChunkPlan:      chunkPlan,
	})
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("transcode pipeline canceled after encode: %w", err)
	}

	return s.uploadLocalMP4Outputs(ctx, req.targets, localPaths, req.driver, req.meta.Duration)
}

// uploadLocalMP4Outputs 对每个本地 MP4 先 probe，再按文件大小超时上传。
// 串行上传，避免重编码刚结束又对 MinIO 打出多路上传压力。
func (s *TranscodeService) uploadLocalMP4Outputs(
	ctx context.Context,
	targets []mediavo.TranscodeTarget,
	localPaths map[string]string,
	driver mediaport.StorageDriver,
	sourceDuration int,
) (map[string]uploadResult, error) {
	resultMap := make(map[string]uploadResult, len(targets))

	var firstErr error

	for i := range targets {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			break
		}

		target := targets[i]
		localPath := localPaths[target.ID]

		res, err := s.uploadOneLocalMP4(ctx, target, localPath, driver, sourceDuration)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}

			s.logger.Error("upload local mp4 output failed",
				zap.String("targetId", target.ID),
				zap.String("path", localPath),
				zap.Error(err),
			)

			continue
		}

		resultMap[target.ID] = res
	}

	if firstErr != nil {
		return resultMap, firstErr
	}

	if len(resultMap) != len(targets) {
		return resultMap, fmt.Errorf("MP4 pipeline produced %d/%d outputs", len(resultMap), len(targets))
	}

	return resultMap, nil
}

// uploadOneLocalMP4 校验并上传单个本地 MP4（stat + probe + 按体积超时的 SaveTranscodeVideo）。
func (s *TranscodeService) uploadOneLocalMP4(
	ctx context.Context,
	target mediavo.TranscodeTarget,
	localPath string,
	driver mediaport.StorageDriver,
	sourceDuration int,
) (uploadResult, error) {
	empty := uploadResult{targetID: target.ID, resolution: target.Resolution}

	fi, err := os.Stat(localPath)
	if err != nil {
		return empty, fmt.Errorf("stat transcoded output %s: %w", target.ID, err)
	}

	if fi.Size() <= 0 {
		return empty, fmt.Errorf("transcoded output %s is empty", target.ID)
	}

	meta, err := s.prober.ProbeVideo(ctx, localPath)
	if err != nil {
		return empty, fmt.Errorf("probe transcoded output %s failed: %w", target.ID, err)
	}

	if err := mediavo.ValidateTranscodedDuration(sourceDuration, meta.Duration); err != nil {
		return empty, fmt.Errorf("validate transcoded output %s: %w", target.ID, err)
	}

	f, err := os.Open(localPath) //nolint:gosec // 路径来自本进程生成的断点目录产物
	if err != nil {
		return empty, fmt.Errorf("open transcoded output %s: %w", target.ID, err)
	}

	defer func() { _ = f.Close() }()

	uploadTimeout := mediaruntime.CalcUploadTimeout(fi.Size())

	uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	url, size, err := driver.SaveTranscodeVideo(uploadCtx, target.RelativePath, fi.Size(), f, "video/mp4", target.Label)
	if err != nil {
		if errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
			return empty, fmt.Errorf("%w after %s for %s (%d bytes): %w", mediaerrors.ErrTranscodeUploadTimeout, uploadTimeout, target.ID, fi.Size(), err)
		}

		return empty, fmt.Errorf("upload transcoded output %s: %w", target.ID, err)
	}

	if size <= 0 {
		return empty, fmt.Errorf("transcoded output %s uploaded empty", target.ID)
	}

	return uploadResult{
		targetID:   target.ID,
		resolution: target.Resolution,
		fileURL:    url,
		fileSize:   size,
		fileName:   target.FileName,
		bitrate:    meta.Bitrate,
	}, nil
}

// cleanupFailedNormalizedTargets 编码整体失败后删除已上传的规范化源产物；
// 原始输入对象（与规范化目标同名时）永远不参与清理。
func (s *TranscodeService) cleanupFailedNormalizedTargets(
	ctx context.Context,
	srcPath string,
	targets []mediavo.TranscodeTarget,
	driver mediaport.StorageDriver,
) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	for _, target := range targets {
		if !target.IsSource ||
			strings.TrimSpace(target.RelativePath) == "" || mediavo.SourceTargetMatchesInput(srcPath, target.RelativePath) {
			continue
		}

		if err := driver.DeleteStoredFile(cleanupCtx, target.RelativePath); err != nil {
			s.logger.Warn("cleanup failed normalized source output",
				zap.Error(err),
				zap.String("path", target.RelativePath),
			)
		}
	}
}
