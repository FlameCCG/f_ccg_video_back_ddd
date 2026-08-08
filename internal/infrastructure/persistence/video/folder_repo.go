package video

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// FolderRepository 收藏夹与收藏关系仓储的 GORM 实现。
type FolderRepository struct {
	db *gorm.DB
}

var _ domain.FolderRepository = (*FolderRepository)(nil)

// NewFolderRepository 构造收藏夹仓储。
func NewFolderRepository(db *gorm.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

// FindDefault 读取用户默认收藏夹；不存在时返回 (nil, nil)。
func (r *FolderRepository) FindDefault(ctx context.Context, userID uint) (*domain.FavoriteFolder, error) {
	var row po.FavoriteFolder

	err := gormx.DB(ctx, r.db).
		Where("user_id = ? AND is_default = ?", userID, true).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainFolder(&row), nil
}

// Create 创建收藏夹并回填 ID。
func (r *FolderRepository) Create(ctx context.Context, folder *domain.FavoriteFolder) error {
	if folder == nil {
		return domain.ErrParamInvalid
	}

	row := toPOFolder(folder)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	folder.ID = row.ID

	return nil
}

// FindByIDAndUser 读取用户名下的收藏夹；不存在时返回 ErrRecordNotFound。
func (r *FolderRepository) FindByIDAndUser(
	ctx context.Context,
	userID, folderID uint,
) (*domain.FavoriteFolder, error) {
	var row po.FavoriteFolder
	if err := gormx.DB(ctx, r.db).
		Where("id = ? AND user_id = ?", folderID, userID).
		Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}

	return toDomainFolder(&row), nil
}

// Update 更新收藏夹（目前仅名称）。
func (r *FolderRepository) Update(ctx context.Context, folder *domain.FavoriteFolder) error {
	if folder == nil || folder.ID == 0 {
		return domain.ErrParamInvalid
	}

	return gormx.DB(ctx, r.db).
		Model(&po.FavoriteFolder{}).
		Where("id = ?", folder.ID).
		Update("name", folder.Name).Error
}

// DeleteWithVideos 删除收藏夹：先扣减受影响视频的 favorite_count，再清关联与夹本体。
//
// 受影响视频 = 该夹内、且用户其它夹中不再收藏的视频（与原 collectFavoriteCountAffectedVideoIDs 一致）。
func (r *FolderRepository) DeleteWithVideos(
	ctx context.Context,
	userID uint,
	folder *domain.FavoriteFolder,
) ([]uint, error) {
	if folder == nil {
		return nil, domain.ErrParamInvalid
	}

	db := gormx.DB(ctx, r.db)

	affectedVideoIDs, err := collectFavoriteCountAffectedVideoIDs(db, userID, folder.ID)
	if err != nil {
		return nil, err
	}

	if len(affectedVideoIDs) > 0 {
		if updateErr := db.Model(&po.Video{}).
			Where("id IN ?", affectedVideoIDs).
			Update(
				"favorite_count",
				gorm.Expr("CASE WHEN favorite_count > 0 THEN favorite_count - 1 ELSE 0 END"),
			).Error; updateErr != nil {
			return nil, updateErr
		}
	}

	if delErr := db.
		Where("favorite_folder_id = ?", folder.ID).
		Delete(&po.FavoriteFolderVideo{}).Error; delErr != nil {
		return nil, delErr
	}

	if delErr := db.Delete(toPOFolder(folder)).Error; delErr != nil {
		return nil, delErr
	}

	return affectedVideoIDs, nil
}

// collectFavoriteCountAffectedVideoIDs 找出删夹后应扣减 favorite_count 的视频 ID。
func collectFavoriteCountAffectedVideoIDs(db *gorm.DB, userID, folderID uint) ([]uint, error) {
	var folderVideoIDs []uint
	if err := db.Model(&po.FavoriteFolderVideo{}).
		Where("favorite_folder_id = ?", folderID).
		Distinct().
		Pluck("video_id", &folderVideoIDs).Error; err != nil {
		return nil, err
	}

	if len(folderVideoIDs) == 0 {
		return nil, nil
	}

	var remainingVideoIDs []uint
	if err := db.Model(&po.FavoriteFolderVideo{}).
		Joins("JOIN favorite_folders ON favorite_folders.id = favorite_folder_videos.favorite_folder_id").
		Where(
			"favorite_folders.user_id = ? AND favorite_folder_videos.favorite_folder_id <> ? "+
				"AND favorite_folder_videos.video_id IN ?",
			userID, folderID, folderVideoIDs,
		).
		Distinct().
		Pluck("favorite_folder_videos.video_id", &remainingVideoIDs).Error; err != nil {
		return nil, err
	}

	remainingSet := make(map[uint]struct{}, len(remainingVideoIDs))
	for _, videoID := range remainingVideoIDs {
		remainingSet[videoID] = struct{}{}
	}

	affected := make([]uint, 0, len(folderVideoIDs))
	seen := make(map[uint]struct{}, len(folderVideoIDs))

	for _, videoID := range folderVideoIDs {
		if _, ok := seen[videoID]; ok {
			continue
		}

		seen[videoID] = struct{}{}

		if _, ok := remainingSet[videoID]; !ok {
			affected = append(affected, videoID)
		}
	}

	return affected, nil
}

// LinkExists 判断某收藏夹内是否已含该视频。
func (r *FolderRepository) LinkExists(ctx context.Context, folderID, videoID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.FavoriteFolderVideo{}).
		Where("favorite_folder_id = ? AND video_id = ?", folderID, videoID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// DeleteLink 删除收藏夹与视频的关联。
func (r *FolderRepository) DeleteLink(ctx context.Context, folderID, videoID uint) error {
	return gormx.DB(ctx, r.db).
		Where("favorite_folder_id = ? AND video_id = ?", folderID, videoID).
		Delete(&po.FavoriteFolderVideo{}).Error
}

// EnsureLink 建立收藏夹与视频的关联，返回是否新建。
func (r *FolderRepository) EnsureLink(ctx context.Context, folderID, videoID uint) (bool, error) {
	link := po.FavoriteFolderVideo{FavoriteFolderID: folderID, VideoID: videoID}

	result := gormx.DB(ctx, r.db).Where(link).FirstOrCreate(&link)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// UserFavorited 判断用户是否在任意收藏夹内收藏了该视频。
func (r *FolderRepository) UserFavorited(ctx context.Context, videoID, userID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.FavoriteFolderVideo{}).
		Joins("JOIN favorite_folders ON favorite_folders.id = favorite_folder_videos.favorite_folder_id").
		Where(
			"favorite_folders.user_id = ? AND favorite_folder_videos.video_id = ?",
			userID, videoID,
		).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
