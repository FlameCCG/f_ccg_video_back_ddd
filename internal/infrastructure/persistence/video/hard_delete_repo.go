package video

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"gorm.io/gorm"
)

// hardDeleteBatchSize 单批处理的视频 ID 数量（与原 adminVideoHardDeleteBatchSize 一致）。
const hardDeleteBatchSize = 500

// HardDelete 物理删除视频及其全部关联数据，返回各表删除计数与待清理的存储目标。
//
// 删除顺序、批量大小与 DASH 目录前缀收集逻辑与原 admin_delete_service.go 逐条一致。
// 评论与评论点赞归 comment 上下文，由应用层经 CommentPurger 端口先行清理，
// 因此本方法不触碰 comments / comment_likes。
func (r *Repository) HardDelete(
	ctx context.Context,
	videoIDs []uint,
) (*domain.HardDeleteResult, []domain.StorageDeleteTarget, error) {
	db := gormx.DB(ctx, r.db)
	result := &domain.HardDeleteResult{}

	targets, err := collectStorageDeleteTargets(db, videoIDs)
	if err != nil {
		return nil, nil, err
	}

	danmuIDs, err := pluckUintByBatches(db, &po.Danmu{}, "id", "video_id", videoIDs)
	if err != nil {
		return nil, nil, err
	}

	if len(danmuIDs) > 0 {
		if result.DanmuLikes, err = deleteModelByBatches(db, &po.DanmuLike{}, "danmu_id", danmuIDs); err != nil {
			return nil, nil, err
		}
	}

	if result.Danmus, err = deleteModelByBatches(db, &po.Danmu{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	partIDs, err := pluckUintByBatches(db, &po.Part{}, "id", "video_id", videoIDs)
	if err != nil {
		return nil, nil, err
	}

	if len(partIDs) > 0 {
		if result.PartResources, err = deleteModelByBatches(
			db, &po.VideoPartResource{}, "video_part_id", partIDs,
		); err != nil {
			return nil, nil, err
		}
	}

	if result.VideoParts, err = deleteModelByBatches(db, &po.Part{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	return finishRepositoryHardDelete(videoIDs, db, result, targets)
}

// forEachIDBatch 按固定批量遍历 ID 列表。
func forEachIDBatch(ids []uint, fn func([]uint) error) error {
	for start := 0; start < len(ids); start += hardDeleteBatchSize {
		end := min(start+hardDeleteBatchSize, len(ids))

		if err := fn(ids[start:end]); err != nil {
			return err
		}
	}

	return nil
}

// pluckUintByBatches 分批取某列的 uint 值集合。
func pluckUintByBatches(
	db *gorm.DB,
	model any,
	pluckColumn, whereColumn string,
	ids []uint,
) ([]uint, error) {
	out := make([]uint, 0)
	err := forEachIDBatch(ids, func(batch []uint) error {
		var batchOut []uint
		if err := db.Model(model).Where(whereColumn+" IN ?", batch).Pluck(pluckColumn, &batchOut).Error; err != nil {
			return err
		}

		out = append(out, batchOut...)

		return nil
	})

	return out, err
}

// deleteModelByBatches 分批删除模型行，返回总影响行数。
func deleteModelByBatches(db *gorm.DB, model any, whereColumn string, ids []uint) (int64, error) {
	var total int64

	err := forEachIDBatch(ids, func(batch []uint) error {
		res := db.Where(whereColumn+" IN ?", batch).Delete(model)
		if res.Error != nil {
			return res.Error
		}

		total += res.RowsAffected

		return nil
	})

	return total, err
}

// deleteTableByBatches 分批删除中间表行（无模型），返回总影响行数。
func deleteTableByBatches(db *gorm.DB, table, whereColumn string, ids []uint) (int64, error) {
	var total int64

	err := forEachIDBatch(ids, func(batch []uint) error {
		res := db.Table(table).Where(whereColumn+" IN ?", batch).Delete(nil)
		if res.Error != nil {
			return res.Error
		}

		total += res.RowsAffected

		return nil
	})

	return total, err
}

// collectStorageDeleteTargets 收集待清理的对象存储目标（封面 + 视频资源 + 分P资源）。
func collectStorageDeleteTargets(db *gorm.DB, videoIDs []uint) ([]domain.StorageDeleteTarget, error) {
	var videos []po.Video

	if err := forEachIDBatch(videoIDs, func(batch []uint) error {
		var batchVideos []po.Video
		if err := db.Model(&po.Video{}).
			Select("id", "cover", "storage_type").
			Where("id IN ?", batch).
			Find(&batchVideos).Error; err != nil {
			return err
		}

		videos = append(videos, batchVideos...)

		return nil
	}); err != nil {
		return nil, err
	}

	videoStorage := make(map[uint]enum.VideoStorageType, len(videos))
	targets := make([]domain.StorageDeleteTarget, 0)
	seen := make(map[string]struct{})

	addTarget := func(storageType enum.VideoStorageType, storedPath string) {
		storedPath = strings.TrimSpace(storedPath)
		if storedPath == "" {
			return
		}

		key := string(storageType) + "\x00" + storedPath
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = struct{}{}

		targets = append(targets, domain.StorageDeleteTarget{
			StorageType: storageType,
			Path:        storedPath,
		})
	}

	for _, video := range videos {
		videoStorage[video.ID] = video.StorageType
		addTarget(video.StorageType, video.Cover)
	}

	var resources []po.VideoResource

	if err := forEachIDBatch(videoIDs, func(batch []uint) error {
		var batchResources []po.VideoResource
		if err := db.Model(&po.VideoResource{}).
			Select("video_id", "file_url", "format").
			Where("video_id IN ?", batch).
			Find(&batchResources).Error; err != nil {
			return err
		}

		resources = append(resources, batchResources...)

		return nil
	}); err != nil {
		return nil, err
	}

	for _, resource := range resources {
		if resource.Format == "dash" {
			// DASH: 只加目录前缀（DeleteByPrefix 会清扫 manifest + 所有 rep 分片）
			addDashPrefixTarget(videoStorage[resource.VideoID], resource.FileURL, &targets, seen)

			continue
		}

		addTarget(videoStorage[resource.VideoID], resource.FileURL)
	}

	return finishCollectStorageDeleteTargets(db, videoIDs, videoStorage, &targets, seen, addTarget)
}

// addDashPrefixTarget 为 DASH 资源追加一个目录前缀清理目标。
//
// DeleteByPrefix 会清扫同目录下的 manifest 与所有 rep 分片 mp4。
func addDashPrefixTarget(
	storageType enum.VideoStorageType,
	manifestURL string,
	targets *[]domain.StorageDeleteTarget,
	seen map[string]struct{},
) {
	prefix := domain.DashDirPrefix(manifestURL)
	if prefix == "" {
		return
	}

	key := string(storageType) + "\x00" + prefix
	if _, ok := seen[key]; ok {
		return
	}

	seen[key] = struct{}{}

	*targets = append(*targets, domain.StorageDeleteTarget{
		StorageType:   storageType,
		Path:          prefix,
		DashDirPrefix: prefix,
	})
}

func finishRepositoryHardDelete(videoIDs []uint, db *gorm.DB, result *domain.HardDeleteResult, targets []domain.StorageDeleteTarget) (*domain.HardDeleteResult, []domain.StorageDeleteTarget, error) {
	var err error

	if result.Resources, err = deleteModelByBatches(db, &po.VideoResource{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.Likes, err = deleteModelByBatches(db, &po.VideoLike{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.Coins, err = deleteModelByBatches(db, &po.VideoCoin{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.Favorites, err = deleteModelByBatches(
		db, &po.FavoriteFolderVideo{}, "video_id", videoIDs,
	); err != nil {
		return nil, nil, err
	}

	if result.Histories, err = deleteModelByBatches(
		db, &po.VideoPlayHistory{}, "video_id", videoIDs,
	); err != nil {
		return nil, nil, err
	}

	if result.Reports, err = deleteModelByBatches(db, &po.Report{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.DanmuReports, err = deleteModelByBatches(db, &po.DanmuReport{}, "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.Tags, err = deleteTableByBatches(db, "video_tags", "video_id", videoIDs); err != nil {
		return nil, nil, err
	}

	if result.Videos, err = deleteModelByBatches(db, &po.Video{}, "id", videoIDs); err != nil {
		return nil, nil, err
	}

	return result, targets, nil
}

func finishCollectStorageDeleteTargets(db *gorm.DB, videoIDs []uint, videoStorage map[uint]enum.VideoStorageType, targets *[]domain.StorageDeleteTarget, seen map[string]struct{}, addTarget func(storageType enum.VideoStorageType, storedPath string)) ([]domain.StorageDeleteTarget, error) {
	var parts []po.Part

	if err := forEachIDBatch(videoIDs, func(batch []uint) error {
		var batchParts []po.Part
		if err := db.Model(&po.Part{}).
			Select("id", "video_id", "storage_type").
			Where("video_id IN ?", batch).
			Find(&batchParts).Error; err != nil {
			return err
		}

		parts = append(parts, batchParts...)

		return nil
	}); err != nil {
		return nil, err
	}

	partIDs := make([]uint, 0, len(parts))
	partStorage := make(map[uint]enum.VideoStorageType, len(parts))

	for _, part := range parts {
		partIDs = append(partIDs, part.ID)

		storageType := part.StorageType
		if storageType == "" {
			storageType = videoStorage[part.VideoID]
		}

		partStorage[part.ID] = storageType
	}

	if len(partIDs) == 0 {
		return (*targets), nil
	}

	var partResources []po.VideoPartResource

	if err := forEachIDBatch(partIDs, func(batch []uint) error {
		var batchPartResources []po.VideoPartResource
		if err := db.Model(&po.VideoPartResource{}).
			Select("video_part_id", "file_url", "format").
			Where("video_part_id IN ?", batch).
			Find(&batchPartResources).Error; err != nil {
			return err
		}

		partResources = append(partResources, batchPartResources...)

		return nil
	}); err != nil {
		return nil, err
	}

	for _, resource := range partResources {
		if resource.Format == "dash" {
			addDashPrefixTarget(partStorage[resource.VideoPartID], resource.FileURL, targets, seen)

			continue
		}

		addTarget(partStorage[resource.VideoPartID], resource.FileURL)
	}

	return (*targets), nil
}
