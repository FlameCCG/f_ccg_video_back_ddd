package video

import (
	"context"
	"errors"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// UpdateService 改稿用例服务（原 service/video_service/update_service.go）。
type UpdateService struct {
	txm         tx.Manager
	videos      domain.VideoRepository
	partitions  domain.PartitionRepository
	schedule    ScheduleStore
	authors     *AuthorService
	vectors     *VectorSyncService
	cfgProvider shared.ConfigReader
}

// NewUpdateService 构造改稿用例服务。
func NewUpdateService(
	txm tx.Manager,
	videos domain.VideoRepository,
	partitions domain.PartitionRepository,
	schedule ScheduleStore,
	authors *AuthorService,
	vectors *VectorSyncService,
	cfgProvider shared.ConfigReader,
) *UpdateService {
	return &UpdateService{
		txm:         txm,
		videos:      videos,
		partitions:  partitions,
		schedule:    schedule,
		authors:     authors,
		vectors:     vectors,
		cfgProvider: cfgProvider,
	}
}

// UpdateVideo 更新视频基础信息，需作者本人。
func (s *UpdateService) UpdateVideo(
	ctx context.Context,
	username string,
	opts UpdateVideoOpts,
) (*UpdatedVideo, error) {
	if opts.VideoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	var (
		video     *domain.Video
		tags      []domain.Tag
		partition *domain.Partition
	)
	now := time.Now()
	change := domain.Revision{
		Title: opts.Title, Description: opts.Description, Cover: opts.CoverURL,
		PartitionID: opts.PartitionID, IsOriginal: opts.IsOriginal,
		IsPrivate: opts.IsPrivate, PublishTime: opts.PublishTime,
	}

	// 锁定、领域校验、主体与标签保存共用同一事务。锁在读取快照前取得，
	// 避免作者改稿按旧状态覆盖管理端的删除或审核结果。
	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		var err error
		video, err = s.videos.FindByIDAndAuthorForUpdate(txCtx, opts.VideoID, username)
		if err != nil {
			return err
		}
		if err := video.Revise(username, change, now, s.contentReviewEnabled()); err != nil {
			return err
		}

		if opts.PartitionID != nil {
			partition, err = s.partitions.FindByID(txCtx, *opts.PartitionID)
			if err != nil {
				return errPartitionMissing
			}
			if err := partition.EnsureSubmittable(); err != nil {
				switch {
				case errors.Is(err, domain.ErrPartitionNotFound):
					return errPartitionMissing
				case errors.Is(err, domain.ErrPartitionInactive):
					return errPartitionDisabled
				default:
					return err
				}
			}
		}

		if opts.TagIDs != nil {
			tags, err = s.videos.FindTagsByIDs(txCtx, opts.TagIDs)
			if err != nil {
				return errQueryTagsFailed
			}
		} else {
			tags, err = s.videos.TagsByVideoID(txCtx, video.ID)
			if err != nil {
				return err
			}
		}

		if err := s.videos.SaveRevision(txCtx, video); err != nil {
			return err
		}
		if opts.TagIDs != nil {
			return s.videos.ReplaceTags(txCtx, video.ID, tags)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// 只在可见性或时间发生变更时调整调度；使用领域决策后的实际私密状态。
	if opts.IsPrivate != nil || opts.PublishTime != nil {
		if opts.PublishTime != nil && domain.ShouldScheduleVideoPublish(now, *opts.PublishTime, video.Status == enum.VideoStatusPrivate) {
			_ = s.schedule.Add(ctx, video.ID, *opts.PublishTime)
		} else {
			_ = s.schedule.Remove(ctx, video.ID)
		}
	}

	s.authors.InvalidateVideoAuthorCache(ctx, video.ID)
	s.vectors.SyncAsync(ctx, video.ID, video.Status)

	return buildUpdatedVideo(video, tags, partition), nil
}

func (s *UpdateService) contentReviewEnabled() bool {
	return contentReviewEnabled(s.cfgProvider)
}

// buildUpdatedVideo 组装改稿响应体。
func buildUpdatedVideo(
	video *domain.Video,
	tags []domain.Tag,
	partition *domain.Partition,
) *UpdatedVideo {
	out := &UpdatedVideo{
		ID:             video.ID,
		Title:          video.Title,
		Description:    video.Description,
		Cover:          video.Cover,
		Duration:       video.Duration,
		AuthorUsername: video.AuthorUsername,
		PartitionID:    video.PartitionID,
		Views:          video.Views,
		Likes:          video.Likes,
		CommentCount:   video.CommentCount,
		CoinCount:      video.CoinCount,
		FavoriteCount:  video.FavoriteCount,
		DanmuCount:     video.DanmuCount,
		IsOriginal:     video.IsOriginal,
		StorageType:    video.StorageType,
		Status:         video.Status,
	}

	if tags != nil {
		out.Tags = make([]UpdatedVideoTag, 0, len(tags))
		for _, tag := range tags {
			out.Tags = append(out.Tags, UpdatedVideoTag{ID: tag.ID, Name: tag.Name})
		}
	}

	if partition != nil {
		out.Partition = UpdatedVideoPartition{
			ID:            partition.ID,
			Name:          partition.Name,
			Icon:          partition.Icon,
			SortOrder:     partition.SortOrder,
			IsActive:      partition.IsActive,
			IsSubmittable: partition.IsSubmittable,
		}
	}

	return out
}

// contentReviewEnabled 读取站点内容审核开关（原 IsContentReviewEnabled）。
func contentReviewEnabled(cfgProvider shared.ConfigReader) bool {
	cfg := cfgProvider.Get()

	return cfg != nil &&
		cfg.Site != nil &&
		cfg.Site.ContentReview != nil &&
		cfg.Site.ContentReview.Enable
}
