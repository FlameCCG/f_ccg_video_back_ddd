package video

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/logctx"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// adminVideoStorageCleanupTimeout 对象存储清理的独立超时。
const adminVideoStorageCleanupTimeout = 30 * time.Second

// Delete 管理员删除视频（软删/物理删）。
func (s *AdminVideoService) Delete(
	ctx context.Context,
	videoIDs []uint,
	hard bool,
) (*domain.HardDeleteResult, error) {
	if len(videoIDs) == 0 {
		return nil, domain.ErrVideoIDsRequired
	}

	if !hard {
		return s.softDelete(ctx, videoIDs)
	}

	return s.hardDelete(ctx, videoIDs)
}

func (s *AdminVideoService) softDelete(ctx context.Context, videoIDs []uint) (*domain.HardDeleteResult, error) {
	result := &domain.HardDeleteResult{}

	rows, err := s.markDeleted(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	result.Videos = rows

	s.notifyDeletedVideoTranscodes(ctx, videoIDs)

	s.cleanupVideoCache(ctx, videoIDs)
	s.vectors.SyncBatchAsync(ctx, videoIDs, enum.VideoStatusDeleted)
	logctx.With(ctx, s.logger).Info("admin soft delete video ok", zap.String("event", "video.admin_delete_service.soft_delete.diagnostic"), zap.String("operation", "video.admin_delete_service.soft_delete"), zap.Int("count", int(result.Videos)))

	return result, nil
}

func (s *AdminVideoService) hardDelete(ctx context.Context, videoIDs []uint) (*domain.HardDeleteResult, error) {
	var (
		result         *domain.HardDeleteResult
		storageTargets []domain.StorageDeleteTarget
	)

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if _, err := s.markDeleted(txCtx, videoIDs); err != nil {
			return err
		}

		comments, likes, err := s.comments.PurgeByVideoIDs(txCtx, videoIDs)
		if err != nil {
			return err
		}

		result, storageTargets, err = s.videos.HardDelete(txCtx, videoIDs)
		if err != nil {
			return err
		}

		result.Comments, result.CommentLikes = comments, likes

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.notifyDeletedVideoTranscodes(ctx, videoIDs)

	s.cleanupVideoCache(ctx, videoIDs)
	s.vectors.SyncBatchAsync(ctx, videoIDs, enum.VideoStatusDeleted)
	result.StorageFiles, result.StorageFilesSkipped, result.StorageDeleteFailed = s.deleteStorageFiles(ctx, storageTargets)

	logctx.With(ctx, s.logger).Info("admin hard delete video ok", zap.String("event", "video.admin_delete_service.hard_delete.diagnostic"), zap.String("operation", "video.admin_delete_service.hard_delete"),
		zap.Int64("videos", result.Videos),
		zap.Int64("comments", result.Comments),
		zap.Int64("comment_likes", result.CommentLikes),
		zap.Int64("danmus", result.Danmus),
		zap.Int64("danmu_likes", result.DanmuLikes),
		zap.Int64("parts", result.VideoParts),
		zap.Int64("part_resources", result.PartResources),
		zap.Int64("resources", result.Resources),
		zap.Int64("favorites", result.Favorites),
		zap.Int64("likes", result.Likes),
		zap.Int64("coins", result.Coins),
		zap.Int64("histories", result.Histories),
		zap.Int64("reports", result.Reports),
		zap.Int64("danmu_reports", result.DanmuReports),
		zap.Int64("tags", result.Tags),
		zap.Int64("storage_files", result.StorageFiles),
		zap.Int64("storage_files_skipped", result.StorageFilesSkipped),
		zap.Int64("storage_delete_failed", result.StorageDeleteFailed),
	)

	return result, nil
}

func (s *AdminVideoService) markDeleted(ctx context.Context, videoIDs []uint) (int64, error) {
	var rows int64

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		var err error

		rows, err = s.videos.UpdateStatusBatch(txCtx, videoIDs, enum.VideoStatusDeleted)
		if err != nil {
			return err
		}

		for _, id := range videoIDs {
			if id == 0 {
				continue
			}

			if err := s.schedule.Remove(txCtx, id); err != nil {
				return err
			}
		}

		return nil
	})

	return rows, err
}

func (s *AdminVideoService) notifyDeletedVideoTranscodes(ctx context.Context, videoIDs []uint) {
	if err := s.canceller.NotifyVideoDeleted(ctx, videoIDs); err != nil {
		logctx.With(ctx, s.logger).Warn("notify transcode cancellation failed", zap.String("event", "video.admin_delete_service.notify_deleted_video_transcodes.degraded"), zap.String("operation", "video.admin_delete_service.notify_deleted_video_transcodes"),
			zap.Int("video_count", len(videoIDs)),
			zap.Error(err),
		)
	}
}

// deleteStorageFiles 清理对象存储：DASH 目录按前缀批量清扫，普通文件先做引用检查。
func (s *AdminVideoService) deleteStorageFiles(
	ctx context.Context,
	targets []domain.StorageDeleteTarget,
) (deleted, skipped, failed int64) {
	if len(targets) == 0 {
		return 0, 0, 0
	}

	if ctx == nil {
		ctx = context.TODO()
	} else {
		// 对象存储清理是删除事务后的后台收尾，脱离请求取消但下面仍会设置独立超时。
		ctx = context.WithoutCancel(ctx)
	}

	cleanupCtx, cancel := context.WithTimeout(ctx, adminVideoStorageCleanupTimeout)
	defer cancel()

	drivers := make(map[enum.VideoStorageType]StorageDriver)

	for _, target := range targets {
		// DASH 目录前缀：批量清扫（manifest + 所有 rep 分片），无需 DB 引用检查
		if target.DashDirPrefix != "" {
			driver, err := s.cleanupDriver(drivers, target.StorageType)
			if err != nil {
				failed++

				logctx.With(ctx, s.logger).Warn("resolve storage driver for dash prefix delete failed", zap.String("event", "video.admin_delete_service.delete_storage_files.degraded"), zap.String("operation", "video.admin_delete_service.delete_storage_files"),
					zap.Error(err),
					zap.String("prefix", target.DashDirPrefix),
					zap.String("storage_type", string(target.StorageType)))

				continue
			}

			n, err := driver.DeleteByPrefix(cleanupCtx, target.DashDirPrefix)
			if err != nil {
				failed++

				logctx.With(ctx, s.logger).Warn("delete dash prefix failed", zap.String("event", "video.admin_delete_service.delete_storage_files.degraded"), zap.String("operation", "video.admin_delete_service.delete_storage_files"),
					zap.Error(err),
					zap.String("prefix", target.DashDirPrefix),
					zap.String("storage_type", string(target.StorageType)))
			} else {
				// 前缀目录可能含源片同级对象，失效秒传缓存
				s.uploads.InvalidateUploadCompletedCacheByStoredPath(target.DashDirPrefix)
				s.uploads.InvalidateUploadCompletedCacheByStoredPath(target.Path)

				deleted += n
			}

			continue
		}

		stillReferenced, err := s.videos.StoragePathStillReferenced(cleanupCtx, target.Path)
		if err != nil {
			failed++

			logctx.With(ctx, s.logger).Warn("check storage reference before delete failed", zap.String("event", "video.admin_delete_service.delete_storage_files.degraded"), zap.String("operation", "video.admin_delete_service.delete_storage_files"),
				zap.Error(err),
				zap.String("path", target.Path),
				zap.String("storage_type", string(target.StorageType)))

			continue
		}

		if stillReferenced {
			skipped++

			continue
		}

		driver, resolveErr := s.cleanupDriver(drivers, target.StorageType)
		if resolveErr != nil {
			failed++

			logctx.With(ctx, s.logger).Warn("resolve storage driver for hard delete failed", zap.String("event", "video.admin_delete_service.delete_storage_files.degraded"), zap.String("operation", "video.admin_delete_service.delete_storage_files"),
				zap.Error(resolveErr),
				zap.String("path", target.Path),
				zap.String("storage_type", string(target.StorageType)))

			continue
		}

		if err := driver.DeleteStoredFile(cleanupCtx, target.Path); err != nil {
			failed++

			logctx.With(ctx, s.logger).Warn("delete video storage file failed", zap.String("event", "video.admin_delete_service.delete_storage_files.degraded"), zap.String("operation", "video.admin_delete_service.delete_storage_files"),
				zap.Error(err),
				zap.String("path", target.Path),
				zap.String("storage_type", string(target.StorageType)))

			continue
		}

		// 源片被硬删后必须失效秒传缓存，否则 status 仍 completed → 发布时报「请先完成视频上传」
		s.uploads.InvalidateUploadCompletedCacheByStoredPath(target.Path)

		deleted++
	}

	return deleted, skipped, failed
}

// cleanupVideoCache 删除详情 meta、失效作者缓存并移出热度 ZSet。
func (s *AdminVideoService) cleanupVideoCache(ctx context.Context, videoIDs []uint) {
	if !s.metaCache.Available() || len(videoIDs) == 0 {
		return
	}

	members := make([]uint, 0, len(videoIDs))

	for _, id := range videoIDs {
		if id == 0 {
			continue
		}

		s.metaCache.Delete(ctx, id)
		members = append(members, id)
		s.authors.InvalidateVideoAuthorCache(ctx, id)
	}

	if len(members) > 0 {
		s.hotRank.Remove(ctx, members)
	}
}

// logAdminRestore 记录恢复上架结果（抽出便于 admin_video_service.go 复用）。
func (s *AdminVideoService) logAdminRestore(rows int64) {
	s.logger.Info("admin restore video ok", zap.String("event", "video.admin_delete_service.log_admin_restore.diagnostic"), zap.String("operation", "video.admin_delete_service.log_admin_restore"), zap.Int64("count", rows))
}

func (s *AdminVideoService) cleanupDriver(drivers map[enum.VideoStorageType]StorageDriver, kind enum.VideoStorageType) (StorageDriver, error) {
	if driver, ok := drivers[kind]; ok {
		return driver, nil
	}

	driver, err := s.storage.ResolveByType(kind)
	if err != nil {
		return nil, err
	}

	drivers[kind] = driver

	return driver, nil
}
