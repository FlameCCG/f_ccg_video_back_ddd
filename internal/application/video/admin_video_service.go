package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// AdminVideoService 管理端视频用例服务
// （原 service/video_service/{admin_video_service.go,admin_restore_service.go,
// admin_delete_service.go}）。
type AdminVideoService struct {
	txm       tx.Manager
	videos    domain.VideoRepository
	query     AdminVideoQuery
	stats     *StatsService
	authors   *AuthorService
	metaCache DetailMetaCache
	hotRank   HotRankCache
	schedule  ScheduleStore
	vectors   *VectorSyncService
	comments  CommentPurger
	canceller TranscodeCanceller
	storage   StorageResolver
	uploads   *UploadService
	logger    *zap.Logger
}

// NewAdminVideoService 构造管理端视频用例服务。
func NewAdminVideoService(
	txm tx.Manager,
	videos domain.VideoRepository,
	query AdminVideoQuery,
	stats *StatsService,
	authors *AuthorService,
	metaCache DetailMetaCache,
	hotRank HotRankCache,
	schedule ScheduleStore,
	vectors *VectorSyncService,
	comments CommentPurger,
	canceller TranscodeCanceller,
	storage StorageResolver,
	uploads *UploadService,
	logger *zap.Logger,
) *AdminVideoService {
	return &AdminVideoService{
		txm:       txm,
		videos:    videos,
		query:     query,
		stats:     stats,
		authors:   authors,
		metaCache: metaCache,
		hotRank:   hotRank,
		schedule:  schedule,
		vectors:   vectors,
		comments:  comments,
		canceller: canceller,
		storage:   storage,
		uploads:   uploads,
		logger:    logger,
	}
}

// List 管理端视频列表。
func (s *AdminVideoService) List(
	ctx context.Context,
	params AdminVideoListParams,
) ([]AdminVideoItem, int64, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	params.Keyword = domain.TrimKeyword(params.Keyword)

	if params.UserID > 0 {
		username := s.authors.UsernameByID(ctx, params.UserID)
		if username == "" {
			return []AdminVideoItem{}, 0, nil
		}

		params.AuthorUsername = username
	}

	items, total, err := s.query.List(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 || len(items) == 0 {
		return []AdminVideoItem{}, total, nil
	}

	videoIDs := make([]uint, 0, len(items))
	for i := range items {
		if items[i].ID > 0 {
			videoIDs = append(videoIDs, items[i].ID)
		}
	}

	statsMap := s.stats.LoadBatch(ctx, videoIDs)

	for i := range items {
		if cached := statsMap[items[i].ID]; cached != nil {
			items[i].Views = cached.Views
			items[i].Likes = cached.Likes
			items[i].CommentCount = cached.CommentCount
			items[i].CoinCount = cached.CoinCount
			items[i].FavoriteCount = cached.FavoriteCount
			items[i].DanmuCount = cached.DanmuCount
		}
	}

	return items, total, nil
}

// Review 管理员审核视频。
//
// 通过（Published）时：若 Redis 仍有未来定时任务，则置为 Scheduled 等待到点发布；否则立即 Published。
// 驳回（Private/Deleted）时取消定时任务。
func (s *AdminVideoService) Review(ctx context.Context, params AdminVideoReviewParams) (int64, error) {
	if len(params.VideoIDs) == 0 {
		return 0, domain.ErrVideoIDsRequired
	}

	if err := domain.ValidateReviewTargetStatus(params.Status); err != nil {
		return 0, err
	}

	reviewIDs, err := s.videos.IDsByStatus(ctx, params.VideoIDs, enum.VideoStatusReview)
	if err != nil {
		return 0, err
	}

	if len(reviewIDs) == 0 {
		return 0, nil
	}

	if params.Status != enum.VideoStatusPublished {
		return s.finalizeReviewRejection(ctx, reviewIDs, params.Status)
	}

	return s.finalizeReviewApproval(ctx, reviewIDs)
}

func (s *AdminVideoService) finalizeReviewRejection(
	ctx context.Context,
	reviewIDs []uint,
	status enum.VideoStatus,
) (int64, error) {
	var affected int64
	for _, id := range reviewIDs {
		if id == 0 {
			continue
		}

		rows, err := s.applyReview(ctx, id, status)
		if err != nil {
			return affected, err
		}
		if rows == 0 {
			continue
		}
		affected += rows
		s.invalidateReviewCaches(ctx, id)
		s.vectors.SyncAsync(ctx, id, status)
	}
	return affected, nil
}

func (s *AdminVideoService) finalizeReviewApproval(ctx context.Context, reviewIDs []uint) (int64, error) {
	var (
		publishedIDs []uint
		scheduledIDs []uint
		affected     int64
	)

	for _, id := range reviewIDs {
		if id == 0 {
			continue
		}

		target := enum.VideoStatusPublished

		rows, err := s.applyReviewTarget(ctx, id, &target)
		if err != nil {
			return affected, err
		}

		if rows == 0 {
			continue
		}

		affected += rows
		s.invalidateReviewCaches(ctx, id)

		if target == enum.VideoStatusScheduled {
			scheduledIDs = append(scheduledIDs, id)

			continue
		}

		publishedIDs = append(publishedIDs, id)
	}

	if len(publishedIDs) > 0 {
		s.vectors.SyncBatchAsync(ctx, publishedIDs, enum.VideoStatusPublished)
	}

	if len(scheduledIDs) > 0 {
		s.vectors.SyncBatchAsync(ctx, scheduledIDs, enum.VideoStatusScheduled)
	}

	return affected, nil
}

func (s *AdminVideoService) applyReview(ctx context.Context, id uint, target enum.VideoStatus) (int64, error) {
	return s.applyReviewTarget(ctx, id, &target)
}

// 审核与作者改期、到期发布使用同一视频行锁；计划读取失败时禁止提前发布。
func (s *AdminVideoService) applyReviewTarget(ctx context.Context, id uint, target *enum.VideoStatus) (int64, error) {
	var rows int64
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, id)
		if err != nil || video == nil || video.Status != enum.VideoStatusReview {
			return err
		}
		if *target == enum.VideoStatusPublished {
			score, exists, err := s.schedule.Score(txCtx, id)
			if err != nil {
				return err
			}
			if exists && score > float64(time.Now().Unix()) {
				*target = enum.VideoStatusScheduled
			}
		}
		rows, err = s.videos.UpdateStatusFrom(txCtx, id, enum.VideoStatusReview, *target)
		if err != nil || rows == 0 {
			return err
		}
		if *target != enum.VideoStatusScheduled {
			return s.schedule.Remove(txCtx, id)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (s *AdminVideoService) invalidateReviewCaches(ctx context.Context, videoID uint) {
	if s.metaCache.Available() {
		s.metaCache.Delete(ctx, videoID)
	}

	s.authors.InvalidateVideoAuthorCache(ctx, videoID)
}

// Restore 管理员批量恢复上架视频。
func (s *AdminVideoService) Restore(ctx context.Context, videoIDs []uint) (int64, error) {
	if len(videoIDs) == 0 {
		return 0, domain.ErrVideoIDsRequired
	}

	rows, err := s.videos.UpdateStatusBatchFrom(ctx, videoIDs, enum.VideoStatusDeleted, enum.VideoStatusPublished)
	if err != nil {
		return 0, err
	}

	if s.metaCache.Available() && rows > 0 {
		for _, id := range videoIDs {
			if id == 0 {
				continue
			}

			s.authors.InvalidateVideoAuthorCache(ctx, id)
		}
	}

	if rows > 0 {
		s.vectors.SyncBatchAsync(ctx, videoIDs, enum.VideoStatusPublished)
	}

	s.logAdminRestore(rows)

	return rows, nil
}
