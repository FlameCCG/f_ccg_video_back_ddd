package media

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// cleanupTimeout 失败/删除后清理残留产物的最长等待（原实现固定 10 秒）。
const cleanupTimeout = 10 * time.Second

// pipelineRunResult 是一条管线的运行结果。
type pipelineRunResult struct {
	name             string
	currentSourceURL string
	err              error
}

// executeConfiguredPipelines 依据配置并发运行 MP4 与 DASH 两条管线（原 executeConfiguredPipelines）。
//
// 返回值：当前源片 URL（源规范化完成后指向规范化产物）、是否应在收尾删除原始源、错误。
// MP4 失败即整任务失败；DASH 在 MP4 也运行时失败只记录日志跳过（与原实现一致）。
func (s *TranscodeService) executeConfiguredPipelines(
	ctx context.Context,
	task domain.TranscodeTask,
	meta domain.VideoMeta,
	policy *domain.TranscodePolicy,
	initialSrcPath string,
	driver domain.StorageDriver,
	tracker *domain.TranscodeProgressTracker,
	ckpt Checkpoint,
) (string, bool, error) {
	plan := domain.PlanPipelines(task, meta, policy)

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
			ctx,
			domain.TranscodePipelineMP4,
			"等待 MP4 Pipeline",
			domain.PipelineResolutionsFromTargets(plan.MP4Targets),
		)
	}

	if len(plan.DashResolutions) > 0 {
		tracker.ConfigurePipeline(
			ctx,
			domain.TranscodePipelineDASH,
			"等待 DASH Pipeline",
			domain.UniqueResolutions(plan.DashResolutions),
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
		case domain.TranscodePipelineMP4:
			mp4Err = result.err
			if result.currentSourceURL != "" {
				currentSourceURL = result.currentSourceURL
			}
		case domain.TranscodePipelineDASH:
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

// logPipelineMode 输出「两者都关闭」两个分支的原始日志（其余模式原实现无额外日志）。
func (s *TranscodeService) logPipelineMode(task domain.TranscodeTask, plan domain.PipelinePlan) {
	switch plan.Mode {
	case domain.PipelineModeSourceOnly:
		s.logger.Info("mp4Enable and dashEnable both false, run source-only normalize",
			zap.Uint("videoId", task.VideoID),
			zap.Uint16("pixelHeight", plan.PixelHeight),
			zap.Uint16("classHeight", plan.ClassHeight),
			zap.Bool("highBitrate", plan.HighBitrate),
		)
	case domain.PipelineModeNone:
		s.logger.Info("mp4Enable and dashEnable both false, source already browser-playable",
			zap.Uint("videoId", task.VideoID),
		)
	case domain.PipelineModeMP4Super, domain.PipelineModeDashOnly:
	}
}

// runMP4SuperPipeline 运行 MP4 多输出管线：L1 跳过已入库目标 → 编码 → 上传 → 落库。
func (s *TranscodeService) runMP4SuperPipeline(
	ctx context.Context,
	task domain.TranscodeTask,
	meta domain.VideoMeta,
	policy *domain.TranscodePolicy,
	srcPath string,
	driver domain.StorageDriver,
	tracker *domain.TranscodeProgressTracker,
	targets []domain.TranscodeTarget,
	ckpt Checkpoint,
) pipelineRunResult {
	tracker.StartPipeline(ctx, domain.TranscodePipelineMP4, "生成 MP4 多输出资源")

	// 断点续传 L1：资源行 + 远端对象都在的目标彻底跳过（不编码、不上传、不落库）。
	saved := 0
	currentSourceURL := srcPath
	pending := make([]domain.TranscodeTarget, 0, len(targets))

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
		tracker.MarkPipelineUploading(ctx, domain.TranscodePipelineMP4, "保存 MP4 资源")
		tracker.CompletePipeline(ctx, domain.TranscodePipelineMP4, nil)

		return pipelineRunResult{name: domain.TranscodePipelineMP4, currentSourceURL: currentSourceURL}
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
	tracker.MarkPipelineUploading(ctx, domain.TranscodePipelineMP4, "保存 MP4 资源")

	if domain.TaskWasDeleted(ctx, runErr) {
		s.cleanupDeletedMP4Outputs(ctx, targets, driver)

		runErr = domain.ErrVideoDeleted
		tracker.CompletePipeline(ctx, domain.TranscodePipelineMP4, runErr)

		return pipelineRunResult{name: domain.TranscodePipelineMP4, currentSourceURL: srcPath, err: runErr}
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

	if domain.TaskWasDeleted(ctx, runErr) {
		s.cleanupDeletedMP4Outputs(ctx, targets, driver)

		runErr = domain.ErrVideoDeleted
		currentSourceURL = srcPath
	}

	tracker.CompletePipeline(ctx, domain.TranscodePipelineMP4, runErr)

	return pipelineRunResult{name: domain.TranscodePipelineMP4, currentSourceURL: currentSourceURL, err: runErr}
}

// runDASHOutputPipeline 运行 DASH 管线：L1 跳过已入库整套 → 编码 → 上传 → 落库。
func (s *TranscodeService) runDASHOutputPipeline(
	ctx context.Context,
	task domain.TranscodeTask,
	meta domain.VideoMeta,
	policy *domain.TranscodePolicy,
	srcPath string,
	driver domain.StorageDriver,
	tracker *domain.TranscodeProgressTracker,
	plan domain.PipelinePlan,
	ckpt Checkpoint,
) pipelineRunResult {
	tracker.StartPipeline(ctx, domain.TranscodePipelineDASH, "生成 DASH 自适应流")

	// 断点续传 L1：自适应资源行 + manifest 对象都在 → 整套 DASH 跳过。
	// manifest 最后上传，它存在即代表各 representation 均已完整上传。
	if s.artifacts.DASHPersisted(ctx, task, driver) {
		s.logger.Info("dash output already persisted, skip transcode",
			zap.Uint("videoId", task.VideoID),
			zap.Uint("partId", task.VideoPartID),
		)
		tracker.CompletePipeline(ctx, domain.TranscodePipelineDASH, nil)

		return pipelineRunResult{name: domain.TranscodePipelineDASH}
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

	if domain.TaskWasDeleted(ctx, err) {
		s.cleanupDeletedDASHOutputs(ctx, task, driver)

		err = domain.ErrVideoDeleted
	}

	tracker.CompletePipeline(ctx, domain.TranscodePipelineDASH, err)

	return pipelineRunResult{name: domain.TranscodePipelineDASH, err: err}
}

// cleanupDeletedMP4Outputs 视频被删除时清理已上传的 MP4 产物。
func (s *TranscodeService) cleanupDeletedMP4Outputs(
	ctx context.Context,
	targets []domain.TranscodeTarget,
	driver domain.StorageDriver,
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
	task domain.TranscodeTask,
	driver domain.StorageDriver,
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
