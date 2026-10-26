package mq

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// EncodeMP4Targets 产出（或复用）各 MP4 目标的本地产物，返回 targetID → 已提交产物路径
// （原 runFFmpegPipeline 的编码部分）。
//
// 断点续传 L2：已提交且校验有效的产物跳过编码；其余进入本次 FFmpeg。
// 分段计划命中时走块级编码 + 拼装，中断只重编当前块。
func (e *TranscodeEngine) EncodeMP4Targets(
	ctx context.Context,
	req mediaapp.MP4EncodeRequest,
) (map[string]string, error) {
	if len(req.Targets) == 0 {
		return nil, errors.New("no transcode targets")
	}

	ckpt, releaseCkpt, err := ensureTranscodeCheckpoint(asCheckpoint(req.Checkpoint), e.logger)
	if err != nil {
		return nil, err
	}

	defer releaseCkpt()

	// 深拷贝目标：编码阶段会回填 MapLabel，不能改动调用方共享状态。
	targets := append([]media.TranscodeTarget(nil), req.Targets...)

	validate := mp4CheckpointValidator(req.SourceDuration, e.logger)
	localPaths := make([]string, len(targets))
	outputNames := make([]string, len(targets))
	pending := make([]int, 0, len(targets))

	for i := range targets {
		outputNames[i] = outputFileName(targets[i], i)

		if committedPath, ok := reusableCommittedOutput(ctx, ckpt, targets[i], validate); ok {
			localPaths[i] = committedPath

			continue
		}

		pending = append(pending, i)
	}

	if len(pending) < len(targets) {
		logctx.With(ctx, e.logger).Info("mp4 targets reused from transcode checkpoint", zap.String("event", "mq.transcode_encode.encode_mp4_targets.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_mp4_targets"),
			zap.Uint("video_id", req.Task.VideoID),
			zap.Uint("part_id", req.Task.VideoPartID),
			zap.Int("reused", len(targets)-len(pending)),
			zap.Int("pending", len(pending)),
		)
	}

	switch {
	case len(pending) > 0 && len(req.ChunkPlan) > 0:
		if err := e.encodeChunkedMP4(ctx, req, ckpt, targets, outputNames, localPaths, pending); err != nil {
			return nil, err
		}
	case len(pending) > 0:
		if err := e.encodeDirectMP4(ctx, req, ckpt, targets, outputNames, localPaths, pending, validate); err != nil {
			return nil, err
		}
	}

	results := make(map[string]string, len(targets))
	for i := range targets {
		results[targets[i].ID] = localPaths[i]
	}

	return results, nil
}

// encodeDirectMP4 单次多输出 FFmpeg：输出先写 attempt 子目录，逐文件校验后原子提交。
func (e *TranscodeEngine) encodeDirectMP4(
	ctx context.Context,
	req mediaapp.MP4EncodeRequest,
	ckpt *transcodeCheckpoint,
	targets []media.TranscodeTarget,
	outputNames, localPaths []string,
	pending []int,
	validate transcodeOutputValidator,
) error {
	attemptDir, err := ckpt.AttemptDir("mp4")
	if err != nil {
		return fmt.Errorf("create mp4 attempt dir failed: %w", err)
	}

	defer func() { _ = os.RemoveAll(attemptDir) }()

	encTargets := make([]media.TranscodeTarget, 0, len(pending))
	encOutputs := make([]string, 0, len(pending))

	for _, idx := range pending {
		encTargets = append(encTargets, targets[idx])
		encOutputs = append(encOutputs, filepath.Join(attemptDir, outputNames[idx]))
	}

	useHardwareScale := req.Policy.UseHardwareScale(req.UseGPU)
	filter := buildFilterComplex(encTargets, req.UseGPU, useHardwareScale)
	args := buildFFmpegArgsForOutputs(
		req.Policy, req.SourcePath, filter, encTargets, encOutputs,
		req.UseGPU, useHardwareScale, req.SourceBitrate, req.SourceCodec, nil,
	)

	srcBytes := sourceBytesHint(req.SourcePath)
	encodeTimeout := media.CalcPipelineEncodeTimeout(req.SourceDuration, len(encTargets), req.UseGPU, srcBytes)
	stallLimit := media.CalcProgressStallTimeout(req.SourceDuration)

	encodeCtx, cancelEncode := context.WithTimeout(ctx, encodeTimeout)
	defer cancelEncode()

	runCtx, cancelRun, wrapProgress := media.WithProgressStallGuard(encodeCtx, stallLimit, nil)
	defer cancelRun()

	cmd := exec.CommandContext(runCtx, "ffmpeg", args...) // #nosec G204 -- Executes a fixed ffmpeg/ffprobe binary directly, with generated argv and no shell expansion.
	cmd.Stdout = io.Discard

	tracker := media.ProgressTrackerFromContext(ctx)
	progressCb := wrapProgress(progressCbForPipeline(ctx, tracker, media.TranscodePipelineMP4))
	stderrBuf, startProgressRead, waitProgress := attachFFmpegProgress(cmd, progressCb)

	logctx.With(ctx, e.logger).Info("ffmpeg multi-output pipeline uses checkpoint attempt dir", zap.String("event", "mq.transcode_encode.encode_direct_mp4.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_direct_mp4"),
		zap.Uint("video_id", req.Task.VideoID),
		zap.Int("targets", len(encTargets)),
		zap.Int("reused", len(targets)-len(pending)),
		zap.Bool("use_gpu", req.UseGPU),
		zap.Duration("encode_timeout_ms", encodeTimeout),
		zap.Duration("progress_stall_limit_ms", stallLimit),
		zap.Int64("source_bytes", srcBytes),
		zap.String("attempt_dir", attemptDir),
	)

	if err := cmd.Start(); err != nil {
		waitProgress()

		return fmt.Errorf("start ffmpeg pipeline failed: %w", err)
	}

	startProgressRead()

	waitErr := cmd.Wait()

	waitProgress()

	if waitErr != nil {
		return media.ClassifyFFmpegCtxError(runCtx, waitErr, encodeTimeout, stderrBuf.String())
	}

	// 逐文件校验后原子提交：提交即「已编码」标记，之后的上传失败不会再触发重编码。
	for k, idx := range pending {
		if err := validate(ctx, encOutputs[k]); err != nil {
			return fmt.Errorf("validate transcoded output %s: %w", targets[idx].ID, err)
		}

		committedPath, err := ckpt.CommitFile(encOutputs[k], outputNames[idx])
		if err != nil {
			return err
		}

		localPaths[idx] = committedPath
	}

	return nil
}

// encodeChunkedMP4 分段路径：按块编码 + 按块断点，拼装出的最终产物提交后进入统一上传。
func (e *TranscodeEngine) encodeChunkedMP4(
	ctx context.Context,
	req mediaapp.MP4EncodeRequest,
	ckpt *transcodeCheckpoint,
	targets []media.TranscodeTarget,
	outputNames, localPaths []string,
	pending []int,
) error {
	encTargets := make([]media.TranscodeTarget, 0, len(pending))

	for _, idx := range pending {
		target := targets[idx]
		// 块级编码与拼装都按文件名寻址，缺失时用统一兜底名。
		target.FileName = outputNames[idx]
		encTargets = append(encTargets, target)
	}

	logctx.With(ctx, e.logger).Info("mp4 pipeline uses chunked encode", zap.String("event", "mq.transcode_encode.encode_chunked_mp4.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_chunked_mp4"),
		zap.Uint("video_id", req.Task.VideoID),
		zap.Uint("part_id", req.Task.VideoPartID),
		zap.Int("targets", len(encTargets)),
		zap.Int("chunks", len(req.ChunkPlan)),
		zap.Bool("use_gpu", req.UseGPU),
	)

	assembled, err := e.runChunkedMP4Targets(ctx, req, ckpt, encTargets)
	if err != nil {
		return err
	}

	for _, idx := range pending {
		p, ok := assembled[targets[idx].ID]
		if !ok {
			return fmt.Errorf("chunked MP4 output missing for %s", targets[idx].ID)
		}

		localPaths[idx] = p
	}

	return nil
}

// EncodeDASHDir 产出（或复用）整套 DASH，返回断点目录中已提交的产物目录
// （原 runDASHPipeline 的 run 闭包中编码与提交部分）。
func (e *TranscodeEngine) EncodeDASHDir(ctx context.Context, req mediaapp.DASHEncodeRequest) (string, error) {
	if len(req.Targets) == 0 {
		return "", errors.New("no dash representations")
	}

	ckpt, releaseCkpt, err := ensureTranscodeCheckpoint(asCheckpoint(req.Checkpoint), e.logger)
	if err != nil {
		return "", err
	}

	defer releaseCkpt()

	// 断点复用：上次尝试已提交的完整 DASH 目录直接进入上传（编码整体跳过）。
	// GPU 编码成功但上传失败时，CPU 回退也会走到这里而不重编。
	if committedDir, ok := reusableCommittedDashDir(ctx, ckpt, req.SourceDuration); ok {
		logctx.With(ctx, e.logger).Info("dash outputs reused from transcode checkpoint", zap.String("event", "mq.transcode_encode.encode_dash_dir.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_dash_dir"),
			zap.Uint("video_id", req.Task.VideoID),
			zap.Uint("part_id", req.Task.VideoPartID),
			zap.String("dir", committedDir),
		)

		return committedDir, nil
	}

	tmpDir, err := ckpt.AttemptDir("dash")
	if err != nil {
		return "", fmt.Errorf("create dash attempt dir failed: %w", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	targets := append([]media.TranscodeTarget(nil), req.Targets...)
	mpdPath := filepath.Join(tmpDir, media.DashManifestName)
	fileHash := req.Task.DashRepresentationPrefix()

	if len(req.ChunkPlan) > 0 {
		if err := e.encodeChunkedDASH(ctx, req, ckpt, targets, tmpDir, mpdPath, fileHash); err != nil {
			return "", err
		}
	} else if err := e.encodeDirectDASH(ctx, req, targets, tmpDir, mpdPath, fileHash); err != nil {
		return "", err
	}

	if err := validateDASHOutput(ctx, tmpDir, req.SourceDuration, e.logger); err != nil {
		return "", fmt.Errorf("validate DASH output: %w", err)
	}

	// 校验通过后整目录原子提交：之后即使上传失败，重试也不再重编 DASH。
	committedDir, commitErr := ckpt.CommitDir(tmpDir, transcodeDashCommittedDirName)
	if commitErr != nil {
		return "", commitErr
	}

	// 完整 DASH 已提交为最终断点单元，块产物即刻回收磁盘。
	discardTranscodeChunks(ckpt, media.TranscodePipelineDASH)

	return committedDir, nil
}

// encodeDirectDASH 单次 FFmpeg 直接产出 manifest 与各 representation。
func (e *TranscodeEngine) encodeDirectDASH(
	ctx context.Context,
	req mediaapp.DASHEncodeRequest,
	targets []media.TranscodeTarget,
	tmpDir, mpdPath, fileHash string,
) error {
	filter := buildFilterComplex(targets, req.UseGPU, false)
	args := buildDASHFFmpegArgs(
		req.Policy, req.SourcePath, filter, targets, mpdPath, fileHash,
		req.UseGPU, req.SourceBitrate, req.SourceCodec, req.HasAudio,
	)

	srcBytes := sourceBytesHint(req.SourcePath)
	encodeTimeout := media.CalcPipelineEncodeTimeout(req.SourceDuration, len(targets), req.UseGPU, srcBytes)
	stallLimit := media.CalcProgressStallTimeout(req.SourceDuration)

	encodeCtx, cancelEncode := context.WithTimeout(ctx, encodeTimeout)
	defer cancelEncode()

	runCtx, cancelRun, wrapProgress := media.WithProgressStallGuard(encodeCtx, stallLimit, nil)
	defer cancelRun()

	cmd := exec.CommandContext(runCtx, "ffmpeg", args...) // #nosec G204 -- Executes a fixed ffmpeg/ffprobe binary directly, with generated argv and no shell expansion.
	// CWD 切到临时目录，初始化文件和分片都在同一目录，manifest 只含相对路径。
	cmd.Dir = tmpDir
	cmd.Stdout = io.Discard

	tracker := media.ProgressTrackerFromContext(ctx)
	progressCb := wrapProgress(progressCbForPipeline(ctx, tracker, media.TranscodePipelineDASH))
	stderrBuf, startProgressRead, waitProgress := attachFFmpegProgress(cmd, progressCb)

	logctx.With(ctx, e.logger).Info("dash pipeline start", zap.String("event", "mq.transcode_encode.encode_direct_dash.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_direct_dash"),
		zap.Uint("video_id", req.Task.VideoID),
		zap.Int("representations", len(targets)),
		zap.Bool("use_gpu", req.UseGPU),
		zap.Duration("encode_timeout_ms", encodeTimeout),
		zap.Duration("progress_stall_limit_ms", stallLimit),
		zap.Int64("source_bytes", srcBytes),
	)

	if err := cmd.Start(); err != nil {
		waitProgress()

		return fmt.Errorf("start ffmpeg dash failed: %w", err)
	}

	startProgressRead()

	waitErr := cmd.Wait()

	waitProgress()

	if waitErr != nil {
		return media.ClassifyFFmpegCtxError(runCtx, waitErr, encodeTimeout, stderrBuf.String())
	}

	return nil
}

// encodeChunkedDASH 分段路径：各 representation 按块编码（强制 seg 网格关键帧），copy 拼进 dash muxer。
func (e *TranscodeEngine) encodeChunkedDASH(
	ctx context.Context,
	req mediaapp.DASHEncodeRequest,
	ckpt *transcodeCheckpoint,
	targets []media.TranscodeTarget,
	tmpDir, mpdPath, fileHash string,
) error {
	segDur := req.Policy.SegDurationOrDefault()

	for i := range targets {
		targets[i].FileName = fmt.Sprintf("rep%d.mp4", i)
	}

	logctx.With(ctx, e.logger).Info("dash pipeline uses chunked encode", zap.String("event", "mq.transcode_encode.encode_chunked_dash.diagnostic"), zap.String("operation", "mq.transcode_encode.encode_chunked_dash"),
		zap.Uint("video_id", req.Task.VideoID),
		zap.Uint("part_id", req.Task.VideoPartID),
		zap.Int("representations", len(targets)),
		zap.Int("chunks", len(req.ChunkPlan)),
		zap.Bool("use_gpu", req.UseGPU),
	)

	perChunk, err := e.encodeChunkedTargets(ctx, chunkedEncodeRequest{
		pipelineName:   media.TranscodePipelineDASH,
		task:           req.Task,
		policy:         req.Policy,
		srcPath:        req.SourcePath,
		targets:        targets,
		useGPU:         req.UseGPU,
		forceKFSec:     segDur,
		sourceBitrate:  req.SourceBitrate,
		sourceDuration: req.SourceDuration,
		sourceCodec:    req.SourceCodec,
		checkpoint:     ckpt,
		plan:           req.ChunkPlan,
	})
	if err != nil {
		return err
	}

	audioPath := ""
	if req.HasAudio {
		if audioPath, err = e.ensureCheckpointAudioTrack(ctx, ckpt, req.SourcePath, req.SourceDuration); err != nil {
			return err
		}
	}

	return e.muxChunkedDASH(ctx, ckpt, targets, perChunk, audioPath, tmpDir, mpdPath, fileHash, segDur)
}
