package mq

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// -----------------------------------------------------------------------------
// 分段转码（chunked encode）的执行侧
//
// 切块计划由领域层确定性推导（media.PlanTranscodeChunks / TranscodePolicy.ChunkPlan）；
// 本文件只负责按计划跑 FFmpeg、提交块产物、编码共享音轨并 `-c copy` 拼装。
//
// 关键约束（与原实现逐字一致）：
//   - 块目录按编码模式(gpu/cpu)隔离：NVENC 与 libx264 参数集不同，绝不允许混拼；
//   - 视频块 `-ss`+`-t` 纯视频编码，音频整轨单独编为共享 audio.m4a，拼装时混入；
//   - 末块不加 `-t`，编到源结尾，吃掉 ffprobe 整数截断的小数尾巴；
//   - 拼装产物走既有校验（probe + 时长 2% 容差）后提交为最终断点单元，
//     成功后块目录即删，失败保留块目录供下次续传。
// -----------------------------------------------------------------------------

// chunkedEncodeRequest 是一次块级编码的参数集合。
type chunkedEncodeRequest struct {
	pipelineName   string
	task           media.TranscodeTask
	policy         *media.TranscodePolicy
	srcPath        string
	targets        []media.TranscodeTarget
	useGPU         bool
	forceKFSec     int
	sourceBitrate  int
	sourceDuration int
	sourceCodec    string
	checkpoint     *transcodeCheckpoint
	plan           []media.TranscodeChunk
}

// encodeChunkedTargets 按切块计划逐块编码 targets（纯视频多输出），返回每块的产物路径表
// （原 encodeChunkedTargets）。
//
// 已提交且完整的块直接复用；新编的块经逐文件时长校验后整目录原子提交。
func (e *TranscodeEngine) encodeChunkedTargets(
	ctx context.Context,
	req chunkedEncodeRequest,
) ([]map[string]string, error) {
	if len(req.targets) == 0 || len(req.plan) == 0 {
		return nil, fmt.Errorf("chunked encode requires targets and plan")
	}

	names := make([]string, len(req.targets))
	for i := range req.targets {
		names[i] = outputFileName(req.targets[i], i)
		req.targets[i].FileName = names[i]
	}

	perChunk := make([]map[string]string, len(req.plan))
	reused := 0

	tracker := media.ProgressTrackerFromContext(ctx)
	baseCb := progressCbForPipeline(ctx, tracker, req.pipelineName)

	for ci, chunk := range req.plan {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("chunked encode canceled at chunk %d: %w", chunk.Index, err)
		}

		dirName := media.ChunkArtifactDirName(req.pipelineName, req.useGPU, chunk.Index)
		if files, ok := committedChunkFiles(req.checkpoint, dirName, names); ok {
			perChunk[ci] = files
			reused++

			if baseCb != nil {
				baseCb(float64(chunk.StartSec + chunk.LenSec))
			}

			continue
		}

		files, err := e.runOneChunk(ctx, req, chunk, dirName, names, baseCb)
		if err != nil {
			return nil, err
		}

		perChunk[ci] = files
	}

	if reused > 0 {
		e.logger.Info("transcode chunks reused from checkpoint",
			zap.Uint("videoId", req.task.VideoID),
			zap.Uint("partId", req.task.VideoPartID),
			zap.String("pipeline", req.pipelineName),
			zap.Int("reused", reused),
			zap.Int("total", len(req.plan)),
		)
	}

	return perChunk, nil
}

// runOneChunk 编码单个块并提交块目录。
func (e *TranscodeEngine) runOneChunk(
	ctx context.Context,
	req chunkedEncodeRequest,
	chunk media.TranscodeChunk,
	dirName string,
	names []string,
	baseCb media.ProgressCallback,
) (map[string]string, error) {
	ckpt := req.checkpoint

	attemptDir, err := ckpt.AttemptDir(req.pipelineName + "-chunk")
	if err != nil {
		return nil, fmt.Errorf("create chunk attempt dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(attemptDir) }()

	outputs := make([]string, len(names))
	for i, name := range names {
		outputs[i] = filepath.Join(attemptDir, name)
	}

	useHardwareScale := req.policy.UseHardwareScale(req.useGPU)
	filter := buildFilterComplex(req.targets, req.useGPU, useHardwareScale)

	span := &media.FFmpegChunkSpan{
		StartSec:              chunk.StartSec,
		LenSec:                chunk.LenSec,
		VideoOnly:             true,
		ForceKeyframeEverySec: req.forceKFSec,
	}
	if media.IsLastChunk(req.plan, chunk) {
		// 末块编到源结尾（不加 -t）：ffprobe 时长整数截断的小数尾巴不能丢。
		span.LenSec = 0
	}

	args := buildFFmpegArgsForOutputs(
		req.policy, req.srcPath, filter, req.targets, outputs,
		req.useGPU, useHardwareScale, req.sourceBitrate, req.sourceCodec, span,
	)

	srcBytes := sourceBytesHint(req.srcPath)

	chunkSrcBytes := int64(0)
	if srcBytes > 0 && req.sourceDuration > 0 {
		chunkSrcBytes = srcBytes * int64(chunk.LenSec) / int64(req.sourceDuration)
	}

	encodeTimeout := media.CalcPipelineEncodeTimeout(chunk.LenSec, len(req.targets), req.useGPU, chunkSrcBytes)
	stallLimit := media.CalcProgressStallTimeout(chunk.LenSec)

	encodeCtx, cancelEncode := context.WithTimeout(ctx, encodeTimeout)
	defer cancelEncode()

	runCtx, cancelRun, wrapProgress := media.WithProgressStallGuard(encodeCtx, stallLimit, e.logProgressStall)
	defer cancelRun()

	cmd := exec.CommandContext(runCtx, "ffmpeg", args...)
	cmd.Stdout = io.Discard

	// 块内 out_time 从 0 起算，叠加块起点还原全片时间轴。
	progressCb := wrapProgress(func(outTimeSec float64) {
		if baseCb != nil {
			baseCb(float64(chunk.StartSec) + outTimeSec)
		}
	})
	stderrBuf, startProgressRead, waitProgress := attachFFmpegProgress(cmd, progressCb)

	e.logger.Info("transcode chunk encode start",
		zap.Uint("videoId", req.task.VideoID),
		zap.Uint("partId", req.task.VideoPartID),
		zap.String("pipeline", req.pipelineName),
		zap.Int("chunk", chunk.Index),
		zap.Int("startSec", chunk.StartSec),
		zap.Int("lenSec", chunk.LenSec),
		zap.Int("targets", len(req.targets)),
		zap.Bool("useGPU", req.useGPU),
		zap.Duration("encodeTimeout", encodeTimeout),
		zap.Duration("progressStallLimit", stallLimit),
	)

	if err := cmd.Start(); err != nil {
		waitProgress()

		return nil, fmt.Errorf("start ffmpeg chunk %d failed: %w", chunk.Index, err)
	}

	startProgressRead()
	waitErr := cmd.Wait()
	waitProgress()

	if waitErr != nil {
		return nil, fmt.Errorf("chunk %d [%ds,+%ds): %w",
			chunk.Index, chunk.StartSec, chunk.LenSec,
			media.ClassifyFFmpegCtxError(runCtx, waitErr, encodeTimeout, stderrBuf.String()),
		)
	}

	for i, out := range outputs {
		meta, err := ProbeVideo(ctx, out, e.logger)
		if err != nil {
			return nil, fmt.Errorf("probe chunk %d output %s: %w", chunk.Index, req.targets[i].ID, err)
		}

		if err := media.ValidateTranscodedDuration(chunk.LenSec, meta.Duration); err != nil {
			return nil, fmt.Errorf("validate chunk %d output %s: %w", chunk.Index, req.targets[i].ID, err)
		}
	}

	committedDir, err := ckpt.CommitDir(attemptDir, dirName)
	if err != nil {
		return nil, err
	}

	files := make(map[string]string, len(names))
	for _, name := range names {
		files[name] = filepath.Join(committedDir, name)
	}

	return files, nil
}

// -----------------------------------------------------------------------------
// 共享音轨
// -----------------------------------------------------------------------------

// ensureCheckpointAudioTrack 确保共享音轨存在：优先复用已提交音轨（校验时长），否则整条编码后提交。
// MP4/DASH 两条管线并发调用，用断点目录内的锁串行化避免重复编码。
func (e *TranscodeEngine) ensureCheckpointAudioTrack(
	ctx context.Context,
	ckpt *transcodeCheckpoint,
	srcPath string,
	sourceDurationSec int,
) (string, error) {
	ckpt.audioMu.Lock()
	defer ckpt.audioMu.Unlock()

	if p, ok := ckpt.CommittedFilePath(media.TranscodeAudioTrackName); ok {
		dur, err := probeContainerDurationSec(ctx, p)
		if err == nil && media.ValidateTranscodedDuration(sourceDurationSec, dur) == nil {
			return p, nil
		}

		e.logger.Warn("committed shared audio track invalid, re-encode",
			zap.String("path", p),
			zap.Error(err),
		)
		ckpt.DiscardCommitted(media.TranscodeAudioTrackName)
	}

	attemptDir, err := ckpt.AttemptDir("audio")
	if err != nil {
		return "", fmt.Errorf("create audio attempt dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(attemptDir) }()

	outPath := filepath.Join(attemptDir, media.TranscodeAudioTrackName)

	encodeCtx, cancel := context.WithTimeout(ctx, media.CalcAudioEncodeTimeout(sourceDurationSec))
	defer cancel()

	var stderr bytes.Buffer

	cmd := exec.CommandContext(encodeCtx, "ffmpeg", buildAudioTrackEncodeArgs(srcPath, outPath)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("encode shared audio track: %w, stderr: %s", err, stderr.String())
	}

	dur, err := probeContainerDurationSec(ctx, outPath)
	if err != nil {
		return "", fmt.Errorf("probe shared audio track: %w", err)
	}

	if err := media.ValidateTranscodedDuration(sourceDurationSec, dur); err != nil {
		return "", fmt.Errorf("validate shared audio track: %w", err)
	}

	return ckpt.CommitFile(outPath, media.TranscodeAudioTrackName)
}

// -----------------------------------------------------------------------------
// 拼装（-c copy）
// -----------------------------------------------------------------------------

// runChunkedMP4Targets 分段路径的 MP4 编码 + 拼装：返回每个 target 的最终提交产物路径
// （原 runChunkedMP4Targets）。
func (e *TranscodeEngine) runChunkedMP4Targets(
	ctx context.Context,
	req mediaapp.MP4EncodeRequest,
	ckpt *transcodeCheckpoint,
	targets []media.TranscodeTarget,
) (map[string]string, error) {
	perChunk, err := e.encodeChunkedTargets(ctx, chunkedEncodeRequest{
		pipelineName:   media.TranscodePipelineMP4,
		task:           req.Task,
		policy:         req.Policy,
		srcPath:        req.SourcePath,
		targets:        targets,
		useGPU:         req.UseGPU,
		forceKFSec:     0,
		sourceBitrate:  req.SourceBitrate,
		sourceDuration: req.SourceDuration,
		sourceCodec:    req.SourceCodec,
		checkpoint:     ckpt,
		plan:           req.ChunkPlan,
	})
	if err != nil {
		return nil, err
	}

	audioPath := ""
	if req.HasAudio {
		audioPath, err = e.ensureCheckpointAudioTrack(ctx, ckpt, req.SourcePath, req.SourceDuration)
		if err != nil {
			return nil, err
		}
	}

	asmDir, err := ckpt.AttemptDir("mp4-assemble")
	if err != nil {
		return nil, fmt.Errorf("create mp4 assemble dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(asmDir) }()

	listNameSanitizer := strings.NewReplacer(":", "-", "/", "-", "\\", "-")
	results := make(map[string]string, len(targets))

	for i := range targets {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("chunked mp4 assemble canceled: %w", err)
		}

		target := &targets[i]
		name := filepath.Base(target.FileName)

		chunkPaths := make([]string, 0, len(perChunk))

		for ci := range perChunk {
			p, ok := perChunk[ci][name]
			if !ok {
				return nil, fmt.Errorf("chunk %d missing output for target %s", ci, target.ID)
			}

			chunkPaths = append(chunkPaths, p)
		}

		listPath, err := writeChunkConcatList(
			asmDir,
			"concat-"+listNameSanitizer.Replace(target.ID)+".txt",
			chunkPaths,
		)
		if err != nil {
			return nil, err
		}

		outPath := filepath.Join(asmDir, name)
		timeout := media.CalcAssembleTimeout(sumFileSizes(append(chunkPaths, audioPath)...))
		args := buildChunkConcatMP4Args(listPath, audioPath, outPath)

		if err := runFFmpegCopyMux(ctx, args, "", timeout, "mp4 "+target.ID); err != nil {
			return nil, err
		}

		meta, err := ProbeVideo(ctx, outPath, e.logger)
		if err != nil {
			return nil, fmt.Errorf("probe assembled output %s: %w", target.ID, err)
		}

		if err := media.ValidateTranscodedDuration(req.SourceDuration, meta.Duration); err != nil {
			return nil, fmt.Errorf("validate assembled output %s: %w", target.ID, err)
		}

		committedPath, err := ckpt.CommitFile(outPath, target.FileName)
		if err != nil {
			return nil, err
		}

		results[target.ID] = committedPath
	}

	// 全部目标已拼装并提交为最终断点单元，块产物即刻回收磁盘。
	discardTranscodeChunks(ckpt, media.TranscodePipelineMP4)

	return results, nil
}

// muxChunkedDASH 把块产物按 representation 拼装并 copy 进 dash muxer，
// 产物落入 tmpDir（后续走既有校验 + 提交 + 上传）。
func (e *TranscodeEngine) muxChunkedDASH(
	ctx context.Context,
	ckpt *transcodeCheckpoint,
	targets []media.TranscodeTarget,
	perChunk []map[string]string,
	audioPath, tmpDir, mpdPath, fileHash string,
	segDur int,
) error {
	// 清单放独立目录：不能落 tmpDir，否则会被当成 DASH 产物一起提交上传。
	listDir, err := ckpt.AttemptDir("dash-lists")
	if err != nil {
		return fmt.Errorf("create dash list dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(listDir) }()

	listPaths := make([]string, len(targets))

	var allChunkFiles []string

	for i := range targets {
		name := filepath.Base(targets[i].FileName)

		chunkPaths := make([]string, 0, len(perChunk))

		for ci := range perChunk {
			p, ok := perChunk[ci][name]
			if !ok {
				return fmt.Errorf("chunk %d missing output for dash rep %s", ci, targets[i].ID)
			}

			chunkPaths = append(chunkPaths, p)
		}

		allChunkFiles = append(allChunkFiles, chunkPaths...)

		listPaths[i], err = writeChunkConcatList(listDir, fmt.Sprintf("rep%d.txt", i), chunkPaths)
		if err != nil {
			return err
		}
	}

	timeout := media.CalcAssembleTimeout(sumFileSizes(append(allChunkFiles, audioPath)...))
	args := buildChunkedDASHMuxArgs(listPaths, audioPath, mpdPath, fileHash, segDur)

	return runFFmpegCopyMux(ctx, args, tmpDir, timeout, "dash")
}
