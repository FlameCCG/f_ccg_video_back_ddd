package media

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	mediaruntime "f_ccg_video_back_ddd/internal/application/media/runtime"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// cleanupTimeout 失败/删除后清理残留产物的最长等待。
const cleanupTimeout = 10 * time.Second

// pipelineRunResult 是一条管线的运行结果。
type pipelineRunResult struct {
	name             string
	currentSourceURL string
	err              error
}

// executeConfiguredPipelines 依据配置并发运行 MP4 与 DASH 两条管线。
//
// 返回值：当前源片 URL（源规范化完成后指向规范化产物）、是否应在收尾删除原始源、错误。
// MP4 失败即整任务失败；DASH 在 MP4 也运行时失败只记录日志跳过。
func (s *TranscodeService) executeConfiguredPipelines(
	ctx context.Context,
	task mediadomain.TranscodeTask,
	meta mediadomain.VideoMeta,
	policy *mediadomain.TranscodePolicy,
	initialSrcPath string,
	driver StorageDriver,
	tracker *mediaruntime.TranscodeProgressTracker,
	ckpt Checkpoint,
) (string, bool, error) {
	plan := mediadomain.PipelinePlanner{Policy: policy}.Plan(task, meta)

	if plan.NotPlayableReason != "" {
		s.logger.Info("source requires browser-compatible normalization",
			zap.Uint("videoId", task.VideoID),
			zap.String("reason", plan.NotPlayableReason),
			zap.String("sourcePath", initialSrcPath),
			zap.String("codec", meta.Codec),
			zap.Uint16("pixelHeight", plan.PixelHeight),
			zap.Uint16("classHeight", plan.ClassHeight),
		)
	}

	s.logPipelineMode(task, plan)

	if len(plan.MP4Targets) > 0 {
		tracker.ConfigurePipeline(
			ctx, mediadomain.TranscodePipelineMP4, "等待 MP4 Pipeline", plan.MP4Resolutions(),
		)
	}

	if len(plan.DashResolutions) > 0 {
		tracker.ConfigurePipeline(
			ctx, mediadomain.TranscodePipelineDASH, "等待 DASH Pipeline", plan.UniqueDashResolutions(),
		)
	}

	jobs := 0
	resultCh := make(chan pipelineRunResult, 2)

	var wg sync.WaitGroup

	if len(plan.MP4Targets) > 0 {
		jobs++

		wg.Add(1)

		go func() {
			defer wg.Done()

			resultCh <- s.runMP4SuperPipeline(
				ctx, task, meta, policy, initialSrcPath, driver, tracker, plan.MP4Targets, ckpt,
			)
		}()
	}

	if len(plan.DashResolutions) > 0 {
		jobs++

		wg.Add(1)

		go func() {
			defer wg.Done()

			resultCh <- s.runDASHOutputPipeline(ctx, task, meta, policy, initialSrcPath, driver, tracker, plan, ckpt)
		}()
	}

	if jobs == 0 {
		return initialSrcPath, false, nil
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	currentSourceURL := initialSrcPath

	var mp4Err, dashErr error

	for result := range resultCh {
		switch result.name {
		case mediadomain.TranscodePipelineMP4:
			mp4Err = result.err
			if result.currentSourceURL != "" {
				currentSourceURL = result.currentSourceURL
			}
		case mediadomain.TranscodePipelineDASH:
			dashErr = result.err
		}
	}

	if mp4Err != nil {
		return currentSourceURL, false, mp4Err
	}

	if dashErr != nil {
		if len(plan.MP4Targets) == 0 {
			return currentSourceURL, false, dashErr
		}

		s.logger.Warn("dash pipeline failed (skipped)", zap.Uint("videoId", task.VideoID), zap.Error(dashErr))
	}

	deleteOriginal := plan.NeedsSourceMP4 && currentSourceURL != initialSrcPath

	return currentSourceURL, deleteOriginal, nil
}

// logPipelineMode 输出 MP4 与 DASH 均关闭时的日志。
func (s *TranscodeService) logPipelineMode(task mediadomain.TranscodeTask, plan mediadomain.PipelinePlan) {
	switch plan.Mode {
	case mediadomain.PipelineModeSourceOnly:
		s.logger.Info("mp4Enable and dashEnable both false, run source-only normalize",
			zap.Uint("videoId", task.VideoID),
			zap.Uint16("pixelHeight", plan.PixelHeight),
			zap.Uint16("classHeight", plan.ClassHeight),
			zap.Bool("highBitrate", plan.HighBitrate),
		)
	case mediadomain.PipelineModeNone:
		s.logger.Info("mp4Enable and dashEnable both false, source already browser-playable",
			zap.Uint("videoId", task.VideoID),
		)
	case mediadomain.PipelineModeMP4Super, mediadomain.PipelineModeDashOnly:
	}
}

// runMP4SuperPipeline 运行 MP4 多输出管线：L1 跳过已入库目标 → 编码 → 上传 → 落库。
//
// 入库记录和存储对象都有效才允许跳过，防止“资源行存在但对象已丢失”被当成任务完成。
// 编码检查点是另一层复用：有本地产物时可跳过编码，但上传和资源保存仍需按状态继续。
func (s *TranscodeService) runMP4SuperPipeline(
	ctx context.Context,
	task mediadomain.TranscodeTask,
	meta mediadomain.VideoMeta,
	policy *mediadomain.TranscodePolicy,
	srcPath string,
	driver StorageDriver,
	tracker *mediaruntime.TranscodeProgressTracker,
	targets []mediadomain.TranscodeTarget,
	ckpt Checkpoint,
) pipelineRunResult {
	tracker.StartPipeline(ctx, mediadomain.TranscodePipelineMP4, "生成 MP4 多输出资源")

	// 断点续传 L1：资源行 + 远端对象都在的目标彻底跳过（不编码、不上传、不落库）。
	saved := 0
	currentSourceURL := srcPath
	pending := make([]mediadomain.TranscodeTarget, 0, len(targets))

	for _, target := range targets {
		if persistedURL, ok := s.artifacts.TargetPersisted(ctx, task, target, driver); ok {
			saved++

			if target.IsSource && persistedURL != "" {
				// 源目标已在历史尝试中完成规范化：接续其 URL，
				// 任务收尾仍能正确删除原始不可播源。
				currentSourceURL = persistedURL
			}

			s.logger.Info("mp4 target already persisted, skip transcode",
				zap.Uint("videoId", task.VideoID),
				zap.Uint("partId", task.VideoPartID),
				zap.String("targetId", target.ID),
			)

			continue
		}

		pending = append(pending, target)
	}

	if len(pending) == 0 {
		tracker.MarkPipelineUploading(ctx, mediadomain.TranscodePipelineMP4, "保存 MP4 资源")
		tracker.CompletePipeline(ctx, mediadomain.TranscodePipelineMP4, nil)

		return pipelineRunResult{name: mediadomain.TranscodePipelineMP4, currentSourceURL: currentSourceURL}
	}

	results, runErr := s.executeVariantPipeline(ctx, variantPipelineRequest{
		task:       task,
		policy:     policy,
		checkpoint: ckpt,
		srcPath:    srcPath,
		targets:    pending,
		driver:     driver,
		meta:       meta,
	})
	tracker.MarkPipelineUploading(ctx, mediadomain.TranscodePipelineMP4, "保存 MP4 资源")

	if mediaruntime.TaskWasDeleted(ctx, runErr) {
		s.cleanupDeletedMP4Outputs(ctx, targets, driver)

		runErr = mediadomain.ErrVideoDeleted
		tracker.CompletePipeline(ctx, mediadomain.TranscodePipelineMP4, runErr)

		return pipelineRunResult{name: mediadomain.TranscodePipelineMP4, currentSourceURL: srcPath, err: runErr}
	}

	sourceName := task.SourceDisplayName()

	for _, target := range pending {
		result, ok := results[target.ID]
		if !ok {
			continue
		}

		var err error
		if target.IsSource {
			err = s.resources.UpdateOriginalResourceFile(
				ctx, task, target.DBResolution, sourceName,
				result.fileURL, result.fileSize, result.bitrate,
			)
			if err == nil {
				currentSourceURL = result.fileURL
			}
		} else {
			err = s.resources.SaveSpecificVideoResource(
				ctx, task, target.DBResolution, sourceName,
				result.fileURL, result.fileSize, result.bitrate, target.IsVIP, false,
			)
		}

		if err != nil {
			if runErr == nil {
				runErr = fmt.Errorf("save MP4 output %s: %w", target.ID, err)
			}

			continue
		}

		saved++
	}

	if saved != len(targets) && runErr == nil {
		runErr = fmt.Errorf("MP4 pipeline produced %d/%d outputs", saved, len(targets))
	}

	if mediaruntime.TaskWasDeleted(ctx, runErr) {
		s.cleanupDeletedMP4Outputs(ctx, targets, driver)

		runErr = mediadomain.ErrVideoDeleted
		currentSourceURL = srcPath
	}

	tracker.CompletePipeline(ctx, mediadomain.TranscodePipelineMP4, runErr)

	return pipelineRunResult{name: mediadomain.TranscodePipelineMP4, currentSourceURL: currentSourceURL, err: runErr}
}

// runDASHOutputPipeline 运行 DASH 管线：L1 跳过已入库整套 → 编码 → 上传 → 落库。
func (s *TranscodeService) runDASHOutputPipeline(
	ctx context.Context,
	task mediadomain.TranscodeTask,
	meta mediadomain.VideoMeta,
	policy *mediadomain.TranscodePolicy,
	srcPath string,
	driver StorageDriver,
	tracker *mediaruntime.TranscodeProgressTracker,
	plan mediadomain.PipelinePlan,
	ckpt Checkpoint,
) pipelineRunResult {
	tracker.StartPipeline(ctx, mediadomain.TranscodePipelineDASH, "生成 DASH 自适应流")

	// 断点续传 L1：自适应资源行 + manifest 对象都在 → 整套 DASH 跳过。
	// manifest 最后上传，它存在即代表各 representation 均已完整上传。
	if s.artifacts.DASHPersisted(ctx, task, driver) {
		s.logger.Info("dash output already persisted, skip transcode",
			zap.Uint("videoId", task.VideoID),
			zap.Uint("partId", task.VideoPartID),
		)
		tracker.CompletePipeline(ctx, mediadomain.TranscodePipelineDASH, nil)

		return pipelineRunResult{name: mediadomain.TranscodePipelineDASH}
	}

	mpdURL, size, err := s.runDASHPipeline(ctx, dashPipelineRequest{
		task:       task,
		policy:     policy,
		checkpoint: ckpt,
		srcPath:    srcPath,
		driver:     driver,
		meta:       meta,
		plan:       plan,
		tracker:    tracker,
	})
	if err == nil && mpdURL != "" {
		err = s.resources.SaveDASHResource(ctx, task, task.SourceDisplayName(), mpdURL, size)
	}

	if mediaruntime.TaskWasDeleted(ctx, err) {
		s.cleanupDeletedDASHOutputs(ctx, task, driver)

		err = mediadomain.ErrVideoDeleted
	}

	tracker.CompletePipeline(ctx, mediadomain.TranscodePipelineDASH, err)

	return pipelineRunResult{name: mediadomain.TranscodePipelineDASH, err: err}
}

// cleanupDeletedMP4Outputs 视频被删除时清理已上传的 MP4 产物。
func (s *TranscodeService) cleanupDeletedMP4Outputs(
	ctx context.Context,
	targets []mediadomain.TranscodeTarget,
	driver StorageDriver,
) {
	if driver == nil {
		return
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	for _, target := range targets {
		if strings.TrimSpace(target.RelativePath) == "" {
			continue
		}

		if err := driver.DeleteStoredFile(cleanupCtx, target.RelativePath); err != nil {
			s.logger.Warn("cleanup deleted video MP4 output failed",
				zap.String("path", target.RelativePath),
				zap.Error(err),
			)
		}
	}
}

// cleanupDeletedDASHOutputs 视频被删除时清理整个 DASH 前缀。
func (s *TranscodeService) cleanupDeletedDASHOutputs(
	ctx context.Context,
	task mediadomain.TranscodeTask,
	driver StorageDriver,
) {
	if driver == nil || strings.TrimSpace(task.OutputPath) == "" {
		return
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()

	prefix := task.DashPrefix()
	if _, err := driver.DeleteByPrefix(cleanupCtx, prefix); err != nil {
		s.logger.Warn("cleanup deleted video DASH output failed",
			zap.String("prefix", prefix),
			zap.Error(err),
		)
	}
}
