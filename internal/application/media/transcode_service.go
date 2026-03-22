package media

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	mediaruntime "f_ccg_video_back_ddd/internal/application/media/runtime"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// TranscodeService 是转码任务的应用编排服务。
//
// 分工：
//   - 纯业务规则（管线选择、码率/分辨率决策、分片计划、超时预算、进度权重）在 internal/domain/media；
//   - ffmpeg/ffprobe 执行、断点目录、Kafka、Redis、对象存储在 infrastructure；
//   - 本服务负责编排：阶段推进、断点续传三层判定、GPU→CPU 回退、产物上传与落库、
//     取消与失败清理、缓存刷新。
type TranscodeService struct {
	paths       ArtifactPaths
	resources   mediadomain.TranscodeResourceRepository
	storage     StorageResolver
	progress    mediaruntime.ProgressStore
	detailCache VideoDetailCacheInvalidator

	prober      MediaProber
	encoder     TranscodeEncoder
	checkpoints CheckpointManager
	stager      SourceStager
	artifacts   ArtifactStateChecker

	gpuCooldown *mediaruntime.GPUCooldown
	cfgProvider shared.ConfigReader
	logger      *zap.Logger
}

// NewTranscodeService 构造转码编排服务。
func NewTranscodeService(
	paths ArtifactPaths,
	resources mediadomain.TranscodeResourceRepository,
	storage StorageResolver,
	progress mediaruntime.ProgressStore,
	detailCache VideoDetailCacheInvalidator,
	prober MediaProber,
	encoder TranscodeEncoder,
	checkpoints CheckpointManager,
	stager SourceStager,
	artifacts ArtifactStateChecker,
	gpuCooldown *mediaruntime.GPUCooldown,
	cfgProvider shared.ConfigReader,
	logger *zap.Logger,
) *TranscodeService {
	return &TranscodeService{
		paths:       paths,
		resources:   resources,
		storage:     storage,
		progress:    progress,
		detailCache: detailCache,
		prober:      prober,
		encoder:     encoder,
		checkpoints: checkpoints,
		stager:      stager,
		artifacts:   artifacts,
		gpuCooldown: gpuCooldown,
		cfgProvider: cfgProvider,
		logger:      logger,
	}
}

// MaxWorkers 返回转码 worker 池容量（未配置时 2），供消费者组件构造并发池。
func (s *TranscodeService) MaxWorkers() int {
	return s.policy().MaxWorkersOrDefault()
}

// TranscodeEnabled 报告转码配置是否已就绪；未配置时消费者视为永久失败。
func (s *TranscodeService) TranscodeEnabled() bool {
	return s.transcodeConf() != nil
}

// StartCheckpointJanitor 启动断点目录孤儿清扫（随 ctx 退出）。
func (s *TranscodeService) StartCheckpointJanitor(ctx context.Context) {
	s.checkpoints.StartCheckpointJanitor(ctx)
}

// ClearProgress 删除进度快照（视频被删除、任务作废时调用）。
func (s *TranscodeService) ClearProgress(ctx context.Context, videoID, partID uint) {
	if s == nil || s.progress == nil {
		return
	}

	s.progress.Clear(ctx, videoID, partID)
}

// NewProgressTracker 为一次任务创建进度跟踪器（消费者在派发前调用）。
func (s *TranscodeService) NewProgressTracker(videoID, partID uint) *mediaruntime.TranscodeProgressTracker {
	return mediaruntime.NewTranscodeProgressTracker(videoID, partID, s.progress, func(snapshot mediadomain.TranscodeProgress) {
		s.logger.Error("transcode progress marked failed",
			zap.Uint("videoId", snapshot.VideoID),
			zap.Uint("partId", snapshot.PartID),
			zap.String("stage", snapshot.Stage),
			zap.Float64("percent", snapshot.Percent),
			zap.Float64("outTimeSec", snapshot.OutTimeSec),
			zap.String("error", snapshot.Error),
		)
	})
}

// transcodeConf 读取当前转码配置（支持热更新；未配置时返回 nil）。
func (s *TranscodeService) transcodeConf() *config.Transcode {
	if s == nil || s.cfgProvider == nil {
		return nil
	}

	cfg := s.cfgProvider.Get()
	if cfg == nil {
		return nil
	}

	return cfg.Transcode
}

// policy 返回当前编码策略（配置缺失时为 nil，领域侧对 nil 安全）。
func (s *TranscodeService) policy() *mediadomain.TranscodePolicy {
	return PolicyFromConfig(s.transcodeConf())
}

// ProcessTask 执行一次完整转码任务。
//
// 一个任务跨数据库、Redis、对象存储和本地检查点，不能以一个数据库事务包住整条管线。
// 分阶段探测任务有效性并在重试时核对产物，让已完成单元有机会复用；这不等于恰好执行一次。
//
// ctx 需已携带进度跟踪器与取消登记（由消费者组件准备）；
// 返回的错误语义：*runtime.PermanentMessageError 表示投 DLQ，
// domain.ErrVideoDeleted 表示任务作废（可提交 offset），其余为可重试失败。
func (s *TranscodeService) ProcessTask(ctx context.Context, task mediadomain.TranscodeTask) (err error) {
	startTime := time.Now()

	if validateErr := task.Validate(); validateErr != nil {
		return mediaruntime.PermanentMessage(validateErr)
	}

	pt := mediaruntime.ProgressTrackerFromContext(ctx)
	if pt == nil {
		pt = s.NewProgressTracker(task.VideoID, task.VideoPartID)
		ctx = mediaruntime.ContextWithProgressTracker(ctx, pt)
	}

	defer func() {
		if mediaruntime.TaskWasDeleted(ctx, err) {
			err = mediadomain.ErrVideoDeleted
			s.ClearProgress(ctx, task.VideoID, task.VideoPartID)

			return
		}

		if err != nil {
			percent, stage, status, outTimeSec := pt.DiagnosticSnapshot()
			s.logger.Warn("transcode task interrupted or failed before success",
				zap.Uint("videoId", task.VideoID),
				zap.Uint("partId", task.VideoPartID),
				zap.String("reason", mediaruntime.ClassifyInterruptReason(ctx, err, false)),
				zap.String("stage", stage),
				zap.String("status", status),
				zap.Float64("percent", percent),
				zap.Float64("outTimeSec", outTimeSec),
				zap.NamedError("err", err),
				zap.NamedError("taskCause", context.Cause(ctx)),
			)
			// 最佳努力写失败标记，供管理端 SSE / 轮询展示。
			pt.Fail(ctx, err.Error())
		}
	}()

	transcodeCfg := s.transcodeConf()
	if transcodeCfg == nil {
		return mediaruntime.PermanentMessage(errors.New("transcode config not configured"))
	}

	policy := PolicyFromConfig(transcodeCfg)

	// 断点续传目录：编码产物先提交到这里，重试时逐单元跳过已完成的编码。
	ckpt, ckptErr := s.checkpoints.OpenCheckpoint(task)
	if ckptErr != nil {
		return ckptErr
	}

	defer func() {
		// 成功 / 视频删除 / 永久失败：不会再有重试，断点整目录删除。
		// 其余（可重试）失败保留断点，下一次尝试复用已编码产物。
		var permanentErr *mediaruntime.PermanentMessageError
		if err == nil || mediaruntime.TaskWasDeleted(ctx, err) || errors.As(err, &permanentErr) || !ckpt.Persistent() {
			ckpt.Remove()
		}
	}()

	s.logger.Debug("transcode payload parsed",
		zap.Uint("videoId", task.VideoID),
		zap.String("sourcePath", task.SourcePath),
		zap.Strings("resolutions", s.stringifyResolutions(task.ResolutionList)),
	)

	if err := s.resources.EnsureTaskActive(ctx, task); err != nil {
		return err
	}

	// 存储驱动（MinIO/Local）
	driver, err := s.storage.ResolveByType(task.StorageType)
	if err != nil {
		return err
	}

	// 源路径是根相对路径（对外契约，保持原样落库），
	// 解析为 ffmpeg 可读的绝对 URL/路径后仅用于 stager 取源。
	srcPath, err := driver.ResolveSource(ctx, task.SourcePath)
	if err != nil {
		return err
	}

	stagingTask := task
	stagingTask.SourcePath = srcPath

	// 准备源（本地/HTTP）；远程源暂存到断点目录，重试时免重复下载
	pt.EnterStage(ctx, mediadomain.TranscodeStagePrepare, "准备源文件")

	preparedSrcPath, cleanup, err := s.stager.PrepareSource(ctx, stagingTask, ckpt)
	if err != nil {
		return err
	}

	defer cleanup()
	pt.CompleteStage(ctx, mediadomain.TranscodeStagePrepare)

	// 探测元信息
	pt.EnterStage(ctx, mediadomain.TranscodeStageProbe, "探测视频元信息")

	meta, err := s.prober.ProbeVideo(ctx, preparedSrcPath)
	if err != nil {
		return err
	}

	pt.SetDuration(meta.Duration)

	if err := s.resources.UpdateDurationFromProbe(ctx, task, meta.Duration); err != nil {
		return fmt.Errorf("update video duration from probe: %w", err)
	}

	pt.CompleteStage(ctx, mediadomain.TranscodeStageProbe)

	s.logProbedMeta(task, meta)

	// 先落库「源视频」真实信息（>=1080P 且高码率会以 1080PHigh 或 2160P 标记）
	isHB1080OrAbove := meta.IsHighBitrate(policy.HighBitrateThresholdOrDefault())

	pt.EnterStage(ctx, mediadomain.TranscodeStageSaveOriginal, "保存源视频资源")

	if err := s.resources.EnsureTaskActive(ctx, task); err != nil {
		return err
	}

	origSize, _ := s.prober.SourceSize(ctx, task) // 取不到也不致命

	if err := s.resources.SaveOriginalResource(ctx, task, meta, task.SourcePath, origSize, isHB1080OrAbove); err != nil {
		return fmt.Errorf("save original resource failed: %w", err)
	}

	// 优先刷新详情缓存，避免发布后缓存空资源占位过久
	s.detailCache.InvalidateVideoDetailMeta(ctx, task.VideoID, task.VideoPartID)
	pt.CompleteStage(ctx, mediadomain.TranscodeStageSaveOriginal)

	pt.EnterStage(ctx, mediadomain.TranscodeStageMultiRes, "执行并行转码 Pipeline")

	if err := s.resources.EnsureTaskActive(ctx, task); err != nil {
		return err
	}

	currentSourceURL, deleteOriginalAtEnd, pipelineErr := s.executeConfiguredPipelines(
		ctx, task, meta, policy, preparedSrcPath, driver, pt, ckpt,
	)
	if pipelineErr != nil {
		return pipelineErr
	}

	if err := s.resources.EnsureTaskActive(ctx, task); err != nil {
		return err
	}

	pt.EnterStage(ctx, mediadomain.TranscodeStageCleanup, "清理与刷新缓存")

	if deleteOriginalAtEnd && s.paths.ShouldDeleteOriginalSource(task.SourcePath, currentSourceURL) {
		if err := driver.DeleteStoredFile(ctx, task.SourcePath); err != nil {
			s.logger.Warn("delete original source failed", zap.Error(err), zap.String("path", task.SourcePath))
		}
	}

	s.detailCache.InvalidateVideoDetailMeta(ctx, task.VideoID, task.VideoPartID)

	s.logger.Info("transcode task completed",
		zap.Uint("videoId", task.VideoID),
		zap.Bool("mp4Enabled", transcodeCfg.Mp4Enable),
		zap.Bool("dashEnabled", transcodeCfg.DashEnable),
		zap.Bool("gpuMode", transcodeCfg.GpuMode),
		zap.Bool("cpuMode", transcodeCfg.CpuMode),
		zap.Bool("isHighBitrate", isHB1080OrAbove),
		zap.Duration("cost", time.Since(startTime)),
	)
	pt.Succeed(ctx)

	return nil
}

// logProbedMeta 输出探测到的源片元信息。
func (s *TranscodeService) logProbedMeta(task mediadomain.TranscodeTask, meta mediadomain.VideoMeta) {
	s.logger.Debug("video metadata probed",
		zap.Uint("videoId", task.VideoID),
		zap.Int("height", meta.Height),
		zap.Int("width", meta.Width),
		zap.Int("bitrate_kbps", meta.Bitrate),
		zap.String("format", meta.Format),
		zap.String("codec", meta.Codec),
		zap.String("videoProfile", meta.VideoProfile),
		zap.String("pixFmt", meta.PixelFormat),
		zap.Int("videoLevel", meta.VideoLevel),
		zap.String("audioCodec", meta.AudioCodec),
		zap.String("audioProfile", meta.AudioProfile),
		zap.String("majorBrand", meta.MajorBrand),
		zap.String("compatibleBrands", meta.CompatibleBrands),
	)
}

func (*TranscodeService) stringifyResolutions(resolutions []uint16) []string {
	result := make([]string, 0, len(resolutions))
	for _, v := range resolutions {
		result = append(result, strconv.Itoa(int(v)))
	}

	return result
}
