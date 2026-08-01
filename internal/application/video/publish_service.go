package video

import (
	"context"
	"fmt"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// PublishService 投稿用例服务（原 api/video_api/publish.go 的业务部分）。
//
// 原实现把投稿编排写在 handler 里（直接用 global.DB 建事务、直接读全局配置、
// 直接 new MQService）；新架构把编排上移到应用层，落库交给仓储，
// 转码投递与向量索引经端口，异步任务经注入的 DetachedTaskGroup。
type PublishService struct {
	videos      domain.VideoRepository
	partitions  domain.PartitionRepository
	uploads     *UploadService
	transcode   TranscodeDispatcher
	vectors     *VectorSyncService
	scheduler   *Scheduler
	cfgProvider *config.Provider
	tasks       *ctxutil.DetachedTaskGroup
	logger      *zap.Logger
}

// NewPublishService 构造投稿用例服务。
func NewPublishService(
	videos domain.VideoRepository,
	partitions domain.PartitionRepository,
	uploads *UploadService,
	transcode TranscodeDispatcher,
	vectors *VectorSyncService,
	scheduler *Scheduler,
	cfgProvider *config.Provider,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *PublishService {
	return &PublishService{
		videos:      videos,
		partitions:  partitions,
		uploads:     uploads,
		transcode:   transcode,
		vectors:     vectors,
		scheduler:   scheduler,
		cfgProvider: cfgProvider,
		tasks:       tasks,
		logger:      logger,
	}
}

// PublishVideo 执行投稿：校验上传 → 落库 → 异步调度与转码投递。
func (s *PublishService) PublishVideo(ctx context.Context, cmd PublishVideoCommand) (*PublishVideoResult, error) {
	now := time.Now()

	publishTime := cmd.PublishTime
	if publishTime.IsZero() || publishTime.Before(now) {
		publishTime = now
	}

	if cmd.IsPrivate {
		publishTime = now
	}

	storageType := s.storageType()

	initialStatus := domain.ResolvePublishVideoStatus(now, &publishTime, cmd.IsPrivate, contentReviewEnabled(s.cfgProvider))

	isMultiPart := len(cmd.Parts) > 0
	if err := s.preparePublishUploads(ctx, &cmd); err != nil {
		return nil, err
	}

	var parts []domain.VideoPart

	if isMultiPart {
		parts = make([]domain.VideoPart, 0, len(cmd.Parts))

		for i, p := range cmd.Parts {
			title := trimSpace(p.Title)
			if title == "" {
				title = domain.DefaultPartTitleFromFileName(p.FileName)
			}

			parts = append(parts, domain.VideoPart{
				Title:       title,
				SortOrder:   i,
				Duration:    0,
				StorageType: storageType,
			})
		}
	} else if trimSpace(cmd.FileName) == "" || trimSpace(cmd.FileHash) == "" {
		return nil, errUploadVideoFirst
	}

	video := &domain.Video{
		Title:          cmd.Title,
		Description:    cmd.Description,
		Cover:          cmd.CoverURL,
		AuthorUsername: cmd.AuthorUsername,
		PartitionID:    cmd.PartitionID,
		IsOriginal:     cmd.IsOriginal,
		Status:         initialStatus,
		Duration:       0,
		StorageType:    storageType,
	}

	// 1. 同步进行数据库保存，获取生成的 VideoID 和 PartID
	persistCtx, cancel := ctxutil.WithDetachedTimeout(ctx, ctxutil.RequestLongTimeout)
	defer cancel()

	if err := s.persistVideo(persistCtx, video, cmd, parts); err != nil {
		return nil, err
	}

	// 2. 将耗时的操作（定时调度、投递转码任务到 MQ）改为异步执行
	s.tasks.Go(ctx, ctxutil.RequestLongTimeout, func(asyncCtx context.Context) {
		s.vectors.SyncAsync(asyncCtx, video.ID, video.Status)

		// 处理定时发布（开启审核时也会写入 ZSet，过审后按点发布）
		if domain.ShouldScheduleVideoPublish(now, publishTime, cmd.IsPrivate) {
			if err := s.scheduler.SchedulePublish(asyncCtx, video.ID, publishTime); err != nil {
				s.logger.Error("schedule publish failed", zap.Error(err))
			}
		}

		// 派发转码任务
		if isMultiPart {
			for idx := range parts {
				order := parts[idx].SortOrder
				if order < 0 || order >= len(cmd.Parts) {
					continue
				}

				input := cmd.Parts[order]
				if err := s.dispatchTranscodeTaskForPart(
					asyncCtx, video.ID, parts[idx].ID,
					input.FilePath, input.FileName, input.FileHash, storageType,
				); err != nil {
					s.logger.Error("dispatch transcode task failed",
						zap.Error(err), zap.Uint("videoId", video.ID), zap.Uint("partId", parts[idx].ID))
				}
			}

			return
		}

		if err := s.dispatchTranscodeTask(asyncCtx, video, cmd, storageType); err != nil {
			s.logger.Error("dispatch transcode task failed", zap.Error(err))
		}
	})

	// 3. 直接返回成功响应给前端
	return &PublishVideoResult{
		VideoID:     video.ID,
		Status:      video.Status,
		PublishTime: publishTime,
		Cover:       video.Cover,
	}, nil
}

// preparePublishUploads 校验源文件名并把已完成上传的对象路径回填到命令里。
func (s *PublishService) preparePublishUploads(ctx context.Context, cmd *PublishVideoCommand) error {
	if cmd == nil {
		return errRequestParamInvalid
	}

	if len(cmd.Parts) > 0 {
		for i := range cmd.Parts {
			if trimSpace(cmd.Parts[i].FileName) == "" || trimSpace(cmd.Parts[i].FileHash) == "" {
				return errUploadAllPartsFirst
			}

			if err := domain.ValidateSourceFileName(cmd.Parts[i].FileName); err != nil {
				return err
			}

			filePath, err := s.uploads.ResolveCompletedUpload(ctx, cmd.Parts[i].FileHash)
			if err != nil {
				return err
			}

			cmd.Parts[i].FilePath = filePath
		}

		return nil
	}

	if trimSpace(cmd.FileName) == "" || trimSpace(cmd.FileHash) == "" {
		return errUploadVideoFirst
	}

	if err := domain.ValidateSourceFileName(cmd.FileName); err != nil {
		return err
	}

	filePath, err := s.uploads.ResolveCompletedUpload(ctx, cmd.FileHash)
	if err != nil {
		return err
	}

	cmd.FilePath = filePath

	return nil
}

// persistVideo 校验标签与分区后，把视频、标签关联、分P与初始资源写入单事务。
func (s *PublishService) persistVideo(
	ctx context.Context,
	video *domain.Video,
	cmd PublishVideoCommand,
	parts []domain.VideoPart,
) error {
	var tags []domain.Tag

	if len(cmd.TagIDs) > 0 {
		found, err := s.videos.FindTagsByIDs(ctx, cmd.TagIDs)
		if err != nil {
			return errQueryTagsFailed
		}

		tags = found

		if len(tags) != len(cmd.TagIDs) {
			existing := make(map[uint]struct{}, len(tags))
			for _, t := range tags {
				existing[t.ID] = struct{}{}
			}

			var missing []uint

			for _, id := range cmd.TagIDs {
				if _, ok := existing[id]; !ok {
					missing = append(missing, id)
				}
			}

			s.logger.Error("标签ID不存在", zap.Uints("missing", missing))

			return fmt.Errorf("标签ID不存在: %v", missing)
		}
	}

	partition, err := s.partitions.FindByID(ctx, cmd.PartitionID)
	if err != nil {
		return domain.ErrPartitionNotFound
	}

	if err := partition.EnsureSubmittable(); err != nil {
		return err
	}

	draft := &domain.PublishDraft{
		Video:  video,
		TagIDs: tagIDsOf(tags),
		Parts:  parts,
	}

	if len(parts) > 0 {
		draft.PartResources = make([]*domain.ResourceDraft, len(parts))
		for idx := range parts {
			if idx >= len(cmd.Parts) {
				continue
			}

			draft.PartResources[idx] = domain.BuildInitialResourceDraft(cmd.Parts[idx].FileName, cmd.Parts[idx].FilePath)
		}
	} else {
		draft.VideoResource = domain.BuildInitialResourceDraft(cmd.FileName, cmd.FilePath)
	}

	return s.videos.Persist(ctx, draft)
}

func (s *PublishService) dispatchTranscodeTask(
	ctx context.Context,
	video *domain.Video,
	cmd PublishVideoCommand,
	storage enum.VideoStorageType,
) error {
	task := TranscodeTask{
		ResolutionList: s.transcodeResolutions(),
		StorageType:    storage,
		SourcePath:     cmd.FilePath,
		FileHash:       cmd.FileHash,
		FileName:       cmd.FileName,
		VideoID:        video.ID,
		OutputPath:     s.transcodeOutputPath(storage, cmd.FileHash),
	}

	if err := s.transcode.PublishTranscodeTask(ctx, task); err != nil {
		return err
	}

	s.transcode.MarkTranscodeQueued(ctx, video.ID, 0)

	return nil
}

func (s *PublishService) dispatchTranscodeTaskForPart(
	ctx context.Context,
	videoID, partID uint,
	filePath, fileName, fileHash string,
	storage enum.VideoStorageType,
) error {
	task := TranscodeTask{
		ResolutionList: s.transcodeResolutions(),
		StorageType:    storage,
		SourcePath:     filePath,
		FileHash:       fileHash,
		FileName:       fileName,
		VideoID:        videoID,
		VideoPartID:    partID,
		OutputPath:     s.transcodeOutputPath(storage, fileHash),
	}

	if err := s.transcode.PublishTranscodeTask(ctx, task); err != nil {
		return err
	}

	s.transcode.MarkTranscodeQueued(ctx, videoID, partID)

	return nil
}

// transcodeResolutions 读取配置的转码分辨率列表，缺省时兜底 360/720/1080。
func (s *PublishService) transcodeResolutions() []uint16 {
	cfg := s.cfgProvider.Get()
	if cfg != nil && cfg.Transcode != nil && len(cfg.Transcode.TranscodeResolutions) > 0 {
		return cfg.Transcode.TranscodeResolutions
	}

	return []uint16{360, 720, 1080}
}

// transcodeOutputPath 按存储类型推导转码产物输出前缀（与原实现逐分支一致）。
func (s *PublishService) transcodeOutputPath(storage enum.VideoStorageType, fileHash string) string {
	switch storage {
	case enum.StorageTypeLocal:
		return joinObjectPath("videos", fileHash)
	case enum.StorageTypeMinio:
		cfg := s.cfgProvider.Get()
		if cfg != nil && cfg.Site != nil && cfg.Site.Storage != nil && cfg.Site.Storage.MinIO != nil {
			return joinObjectPath(cfg.Site.Storage.MinIO.UserUploadPrefix, fileHash)
		}

		return ""
	default:
		return joinObjectPath("videos", fileHash)
	}
}

// storageType 解析当前生效的存储类型（原 pkg.CheckStorageTyped）。
//
// 原实现直接解引用 Site.Storage.Local / MinIO，配置缺失时会 panic；
// 这里补齐 nil 判断后同样兜底为本地存储。
func (s *PublishService) storageType() enum.VideoStorageType {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Storage == nil {
		return enum.StorageTypeLocal
	}

	storage := cfg.Site.Storage
	if storage.Local != nil && storage.Local.Enable {
		return enum.StorageTypeLocal
	}

	if storage.MinIO != nil && storage.MinIO.Enable {
		return enum.StorageTypeMinio
	}

	return enum.StorageTypeLocal
}

// tagIDsOf 提取标签 ID 列表。
func tagIDsOf(tags []domain.Tag) []uint {
	if len(tags) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(tags))
	for _, t := range tags {
		ids = append(ids, t.ID)
	}

	return ids
}
