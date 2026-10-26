package media

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"

	domain "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// TranscodeService 是转码任务的应用编排服务（原 service/mq_service 的 processMessage
// 与 executeConfiguredPipelines 等编排逻辑）。
//
// 分工：
//   - 纯业务规则（管线选择、码率/分辨率决策、分片计划、超时预算、进度权重）在 internal/domain/media；
//   - ffmpeg/ffprobe 执行、断点目录、Kafka、Redis、对象存储在 infrastructure；
//   - 本服务负责编排：阶段推进、断点续传三层判定、GPU→CPU 回退、产物上传与落库、
//     取消与失败清理、缓存刷新。
type TranscodeService struct {
	resources   domain.TranscodeResourceRepository
	storage     domain.StorageResolver
	progress    domain.ProgressStore
	detailCache domain.VideoDetailCacheInvalidator

	prober      Prober
	encoder     TranscodeEncoder
	checkpoints CheckpointManager
	stager      SourceStager
	artifacts   ArtifactStateChecker

	gpuCooldown *domain.GPUCooldown
	cfgProvider shared.ConfigReader
	logger      *zap.Logger
}

// NewTranscodeService 构造转码编排服务。
func NewTranscodeService(
	resources domain.TranscodeResourceRepository,
	storage domain.StorageResolver,
	progress domain.ProgressStore,
	detailCache domain.VideoDetailCacheInvalidator,
	prober Prober,
	encoder TranscodeEncoder,
	checkpoints CheckpointManager,
	stager SourceStager,
	artifacts ArtifactStateChecker,
	gpuCooldown *domain.GPUCooldown,
	cfgProvider shared.ConfigReader,
	logger *zap.Logger,
) *TranscodeService {
	return &TranscodeService{
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
func (s *TranscodeService) NewProgressTracker(videoID, partID uint) *domain.TranscodeProgressTracker {
	return domain.NewTranscodeProgressTracker(videoID, partID, s.progress, nil)
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
func (s *TranscodeService) policy() *domain.TranscodePolicy {
	return PolicyFromConfig(s.transcodeConf())
}

// ProcessTask 执行一次完整转码任务（原 processMessage）。
//
// 一个任务跨数据库、Redis、对象存储和本地检查点，不能以一个数据库事务包住整条管线。
// 分阶段探测任务有效性并在重试时核对产物，让已完成单元有机会复用；这不等于恰好执行一次。
//
// ctx 需已携带进度跟踪器与取消登记（由消费者组件准备）；
// 返回的错误语义与原实现一致：*domain.PermanentMessageError 表示投 DLQ，
// domain.ErrVideoDeleted 表示任务作废（可提交 offset），其余为可重试失败。
func (s *TranscodeService) ProcessTask(ctx context.Context, task domain.TranscodeTask) (err error) {
	if validateErr := task.Validate(); validateErr != nil {
		return domain.PermanentMessage(validateErr)
	}

	pt := domain.ProgressTrackerFromContext(ctx)
	if pt == nil {
		pt = s.NewProgressTracker(task.VideoID, task.VideoPartID)
		ctx = domain.ContextWithProgressTracker(ctx, pt)
	}

	defer func() {
		if domain.TaskWasDeleted(ctx, err) {
			err = domain.ErrVideoDeleted

			s.ClearProgress(ctx, task.VideoID, task.VideoPartID)

			return
		}

		if err != nil {
			// 最佳努力写失败标记，供管理端 SSE / 轮询展示。
			pt.Fail(ctx, err.Error())
		}
	}()

	transcodeCfg := s.transcodeConf()
	if transcodeCfg == nil {
		return domain.PermanentMessage(errors.New("transcode config not configured"))
	}

	policy := PolicyFromConfig(transcodeCfg)

	// 断点续传目录：编码产物先提交到这里，重试时逐单元跳过已完成的编码。
	ckpt, ckptErr := s.checkpoints.OpenCheckpoint(ctx, task)
	if ckptErr != nil {
		return ckptErr
	}

	defer func() {
		// 成功 / 视频删除 / 永久失败：不会再有重试，断点整目录删除。
		// 其余（可重试）失败保留断点，下一次尝试复用已编码产物。
		var permanentErr *domain.PermanentMessageError
		if transcodeServiceProcessTaskCondition(ctx, err, ckpt, permanentErr) {
			ckpt.Remove()
		}
	}()

	logctx.With(ctx, s.logger).Debug("transcode payload parsed", zap.String("event", "media.transcode_service.process_task.diagnostic"), zap.String("operation", "media.transcode_service.process_task"),
		zap.Uint("video_id", task.VideoID),
		zap.String("source_path", task.SourcePath),
		zap.Strings("resolutions", domain.StringifyResolutions(task.ResolutionList)),
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

	return finishTranscodeServiceProcessTask(ctx, s, task, pt, policy, ckpt, driver, srcPath)
}

// logProbedMeta 输出探测到的源片元信息（字段与原实现逐一对应）。
func (s *TranscodeService) logProbedMeta(ctx context.Context, task domain.TranscodeTask, meta domain.VideoMeta) {
	logctx.With(ctx, s.logger).Debug("video metadata probed", zap.String("event", "media.transcode_service.log_probed_meta.diagnostic"), zap.String("operation", "media.transcode_service.log_probed_meta"),
		zap.Uint("video_id", task.VideoID),
		zap.Int("height", meta.Height),
		zap.Int("width", meta.Width),
		zap.Int("bitrate_kbps", meta.Bitrate),
		zap.String("format", meta.Format),
		zap.String("codec", meta.Codec),
		zap.String("video_profile", meta.VideoProfile),
		zap.String("pix_fmt", meta.PixelFormat),
		zap.Int("video_level", meta.VideoLevel),
		zap.String("audio_codec", meta.AudioCodec),
		zap.String("audio_profile", meta.AudioProfile),
		zap.String("major_brand", meta.MajorBrand),
		zap.String("compatible_brands", meta.CompatibleBrands),
	)
}

func transcodeServiceProcessTaskCondition(ctx context.Context, err error, ckpt Checkpoint, permanentErr *domain.PermanentMessageError) bool {
	return err == nil || domain.TaskWasDeleted(ctx, err) || errors.As(err, &permanentErr) || !ckpt.Persistent()
}

func finishTranscodeServiceProcessTask(ctx context.Context, s *TranscodeService, task domain.TranscodeTask, pt *domain.TranscodeProgressTracker, policy *domain.TranscodePolicy, ckpt Checkpoint, driver domain.StorageDriver, srcPath string) error {
	var err error

	stagingTask := task
	stagingTask.SourcePath = srcPath

	// 准备源（本地/HTTP）；远程源暂存到断点目录，重试时免重复下载
	pt.EnterStage(ctx, domain.TranscodeStagePrepare, "准备源文件")

	preparedSrcPath, cleanup, err := s.stager.PrepareSource(ctx, stagingTask, ckpt)
	if err != nil {
		return err
	}

	defer cleanup()

	pt.CompleteStage(ctx, domain.TranscodeStagePrepare)

	// 探测元信息
	pt.EnterStage(ctx, domain.TranscodeStageProbe, "探测视频元信息")

	meta, err := s.prober.ProbeVideo(ctx, preparedSrcPath)
	if err != nil {
		return err
	}

	pt.SetDuration(meta.Duration)

	if errLocal := s.resources.UpdateDurationFromProbe(ctx, task, meta.Duration); errLocal != nil {
		return fmt.Errorf("update video duration from probe: %w", errLocal)
	}

	pt.CompleteStage(ctx, domain.TranscodeStageProbe)

	s.logProbedMeta(ctx, task, meta)

	// 先落库「源视频」真实信息（>=1080P 且高码率会以 1080PHigh 或 2160P 标记）
	isHB1080OrAbove := meta.IsHighBitrate(policy.HighBitrateThresholdOrDefault())

	pt.EnterStage(ctx, domain.TranscodeStageSaveOriginal, "保存源视频资源")

	if errLocal := s.resources.EnsureTaskActive(ctx, task); errLocal != nil {
		return errLocal
	}

	origSize, _ := s.prober.SourceSize(ctx, task) // 取不到也不致命

	if errLocal := s.resources.SaveOriginalResource(ctx, task, meta, task.SourcePath, origSize, isHB1080OrAbove); errLocal != nil {
		return fmt.Errorf("save original resource failed: %w", errLocal)
	}

	// 优先刷新详情缓存，避免发布后缓存空资源占位过久
	s.detailCache.InvalidateVideoDetailMeta(ctx, task.VideoID, task.VideoPartID)
	pt.CompleteStage(ctx, domain.TranscodeStageSaveOriginal)

	pt.EnterStage(ctx, domain.TranscodeStageMultiRes, "执行并行转码 Pipeline")

	if errLocal := s.resources.EnsureTaskActive(ctx, task); errLocal != nil {
		return errLocal
	}

	currentSourceURL, deleteOriginalAtEnd, pipelineErr := s.executeConfiguredPipelines(
		ctx, task, meta, policy, preparedSrcPath, driver, pt, ckpt,
	)
	if pipelineErr != nil {
		return pipelineErr
	}

	if errLocal := s.resources.EnsureTaskActive(ctx, task); errLocal != nil {
		return errLocal
	}

	pt.EnterStage(ctx, domain.TranscodeStageCleanup, "清理与刷新缓存")

	if deleteOriginalAtEnd && domain.ShouldDeleteOriginalSource(task.SourcePath, currentSourceURL) {
		if errLocal := driver.DeleteStoredFile(ctx, task.SourcePath); errLocal != nil {
			logctx.With(ctx, s.logger).Warn("delete original source failed", zap.String("event", "media.transcode_service.process_task.degraded"), zap.String("operation", "media.transcode_service.process_task"), zap.Error(errLocal), zap.String("path", task.SourcePath))
		}
	}

	s.detailCache.InvalidateVideoDetailMeta(ctx, task.VideoID, task.VideoPartID)

	pt.Succeed(ctx)

	return nil
}
