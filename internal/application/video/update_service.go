package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
)

// UpdateService 改稿用例服务（原 service/video_service/update_service.go）。
type UpdateService struct {
	txm         tx.Manager
	videos      domain.VideoRepository
	partitions  domain.PartitionRepository
	schedule    ScheduleStore
	authors     *AuthorService
	vectors     *VectorSyncService
	cfgProvider *config.Provider
}

// NewUpdateService 构造改稿用例服务。
func NewUpdateService(
	txm tx.Manager,
	videos domain.VideoRepository,
	partitions domain.PartitionRepository,
	schedule ScheduleStore,
	authors *AuthorService,
	vectors *VectorSyncService,
	cfgProvider *config.Provider,
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

	video, err := s.videos.FindByIDAndAuthor(ctx, opts.VideoID, username)
	if err != nil {
		return nil, err
	}

	existingTags, err := s.videos.TagsByVideoID(ctx, opts.VideoID)
	if err != nil {
		return nil, err
	}

	// 校验标签
	var (
		tags         []domain.Tag
		tagsProvided bool
	)

	if len(opts.TagIDs) > 0 {
		tagsProvided = true

		tags, err = s.videos.FindTagsByIDs(ctx, opts.TagIDs)
		if err != nil {
			return nil, errQueryTagsFailed
		}
	}

	var (
		partition  *domain.Partition
		nextStatus *enum.VideoStatus
		now        = time.Now()
	)

	if opts.PartitionID != nil {
		partition, err = s.partitions.FindByID(ctx, *opts.PartitionID)
		if err != nil {
			return nil, errPartitionMissing
		}

		if !partition.IsActive {
			return nil, errPartitionDisabled
		}

		if !partition.IsSubmittable {
			return nil, domain.ErrPartitionNotSubmittable
		}
	}

	if opts.IsPrivate != nil {
		status := domain.ResolvePublishVideoStatus(now, opts.PublishTime, *opts.IsPrivate, s.contentReviewEnabled())
		nextStatus = &status
	}

	updates := map[string]any{}
	if opts.Title != nil {
		updates["title"] = *opts.Title
	}

	if opts.Description != nil {
		updates["description"] = *opts.Description
	}

	if opts.PartitionID != nil {
		updates["partition_id"] = *opts.PartitionID
	}

	if opts.IsOriginal != nil {
		updates["is_original"] = *opts.IsOriginal
	}

	if opts.CoverURL != nil {
		updates["cover"] = *opts.CoverURL
	}

	if nextStatus != nil {
		updates["status"] = *nextStatus
	}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if len(updates) > 0 {
			if updateErr := s.videos.UpdateFields(txCtx, video.ID, updates); updateErr != nil {
				return updateErr
			}
		}

		if tagsProvided {
			if tagErr := s.videos.ReplaceTags(txCtx, video.ID, tags); tagErr != nil {
				return tagErr
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if opts.Title != nil {
		video.Title = *opts.Title
	}

	if opts.Description != nil {
		video.Description = *opts.Description
	}

	if opts.PartitionID != nil {
		video.PartitionID = *opts.PartitionID
	}

	if opts.IsOriginal != nil {
		video.IsOriginal = *opts.IsOriginal
	}

	if opts.CoverURL != nil {
		video.Cover = *opts.CoverURL
	}

	if nextStatus != nil {
		video.Status = *nextStatus
	}

	responseTags := existingTags
	if tagsProvided {
		responseTags = tags
	}

	switch {
	case opts.IsPrivate != nil && *opts.IsPrivate:
		_ = s.schedule.Remove(ctx, video.ID)
	case opts.PublishTime != nil && domain.ShouldScheduleVideoPublish(now, *opts.PublishTime, false):
		_ = s.schedule.Add(ctx, video.ID, *opts.PublishTime)
	case opts.PublishTime != nil:
		_ = s.schedule.Remove(ctx, video.ID)
	}

	s.authors.InvalidateVideoAuthorCache(ctx, video.ID)
	s.vectors.SyncAsync(ctx, video.ID, video.Status)

	return buildUpdatedVideo(video, responseTags, partition), nil
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
func contentReviewEnabled(cfgProvider *config.Provider) bool {
	cfg := cfgProvider.Get()

	return cfg != nil &&
		cfg.Site != nil &&
		cfg.Site.ContentReview != nil &&
		cfg.Site.ContentReview.Enable
}
