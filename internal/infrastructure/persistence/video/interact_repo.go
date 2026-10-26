package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LikeRepository 视频点赞关系仓储的 GORM 实现。
type LikeRepository struct {
	db *gorm.DB
}

var _ domain.LikeRepository = (*LikeRepository)(nil)

// NewLikeRepository 构造视频点赞仓储。
func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

// Flip 事务内翻转点赞关系；返回 likes 增量（+1 点赞、-1 取消、0 并发下无变更）。
func (r *LikeRepository) Flip(ctx context.Context, videoID, userID uint) (int64, error) {
	var delta int64

	err := gormx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		like := po.VideoLike{VideoID: videoID, UserID: userID}

		createErr := tx.Create(&like).Error
		if createErr == nil {
			delta = 1

			return nil
		}

		if !gormx.IsDuplicateKeyError(createErr) {
			return createErr
		}

		del := tx.Where("video_id = ? AND user_id = ?", videoID, userID).Delete(&po.VideoLike{})
		if del.Error != nil {
			return del.Error
		}

		if del.RowsAffected == 0 {
			// 并发下已被取消，无需改 stats
			delta = 0

			return nil
		}

		delta = -1

		return nil
	})

	return delta, err
}

// Create 建立点赞关系；唯一键冲突返回 ErrDuplicateLike。
func (r *LikeRepository) Create(ctx context.Context, videoID, userID uint) error {
	if err := gormx.DB(ctx, r.db).
		Create(&po.VideoLike{VideoID: videoID, UserID: userID}).Error; err != nil {
		if gormx.IsDuplicateKeyError(err) {
			return domain.ErrDuplicateLike
		}

		return err
	}

	return nil
}

// Delete 删除点赞关系，返回影响行数。
func (r *LikeRepository) Delete(ctx context.Context, videoID, userID uint) (int64, error) {
	result := gormx.DB(ctx, r.db).
		Where("video_id = ? AND user_id = ?", videoID, userID).
		Delete(&po.VideoLike{})

	return result.RowsAffected, result.Error
}

// Exists 判断用户是否已点赞。
func (r *LikeRepository) Exists(ctx context.Context, videoID, userID uint) (bool, error) {
	var count int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.VideoLike{}).
		Where("video_id = ? AND user_id = ?", videoID, userID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// CoinRepository 视频投币关系仓储的 GORM 实现。
type CoinRepository struct {
	db *gorm.DB
}

var _ domain.CoinRepository = (*CoinRepository)(nil)

// NewCoinRepository 构造视频投币仓储。
func NewCoinRepository(db *gorm.DB) *CoinRepository {
	return &CoinRepository{db: db}
}

// LockByVideoAndUser 以行锁读取投币关系；不存在时返回 ErrRecordNotFound。
func (r *CoinRepository) LockByVideoAndUser(ctx context.Context, videoID, userID uint) error {
	var existing po.VideoCoin

	err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("video_id = ? AND user_id = ?", videoID, userID).
		Take(&existing).Error

	return mapNotFound(err)
}

// Create 建立投币关系；唯一键冲突返回 ErrDuplicateCoin。
func (r *CoinRepository) Create(ctx context.Context, videoID, userID uint, coins int64) error {
	record := po.VideoCoin{VideoID: videoID, UserID: userID, Coins: int(coins)}
	if err := gormx.DB(ctx, r.db).Create(&record).Error; err != nil {
		if gormx.IsDuplicateKeyError(err) {
			return domain.ErrDuplicateCoin
		}

		return err
	}

	return nil
}

// Exists 判断用户是否已对该视频投币。
func (r *CoinRepository) Exists(ctx context.Context, videoID, userID uint) (bool, error) {
	var row struct {
		ID uint
	}

	err := gormx.DB(ctx, r.db).
		Model(&po.VideoCoin{}).
		Select("id").
		Where("video_id = ? AND user_id = ?", videoID, userID).
		Limit(1).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
