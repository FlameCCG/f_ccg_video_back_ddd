package video

import (
	"context"
	"errors"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PinRepository 视频置顶写侧仓储。
//
// 动态模块的「置顶作品」会改写 videos 表的 is_pinned / pinned_at，
// 但 videos 的写侧所有权归 video 上下文，故由本仓储提供，
// dynamic 上下文经其 VideoPinWriter 端口调用。
type PinRepository struct {
	db *gorm.DB
}

// NewPinRepository 构造视频置顶仓储。
func NewPinRepository(db *gorm.DB) *PinRepository {
	return &PinRepository{db: db}
}

// ClearPinnedByAuthor 取消该作者当前已置顶的全部视频。
func (r *PinRepository) ClearPinnedByAuthor(ctx context.Context, authorUsername string) error {
	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("author_username = ? AND is_pinned = ?", authorUsername, true).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}

// SetPinned 仅更新作者名下仍已发布的目标，返回目标是否仍符合条件。
func (r *PinRepository) SetPinned(
	ctx context.Context,
	videoID uint,
	authorUsername string,
	pinnedAt *time.Time,
) (bool, error) {
	return r.updatePin(ctx, videoID, authorUsername, map[string]any{"is_pinned": true, "pinned_at": pinnedAt})
}

// ClearPinned 重复取消仍返回 true；目标缺失、归属或发布状态变化时返回 false。
func (r *PinRepository) ClearPinned(ctx context.Context, videoID uint, authorUsername string) (bool, error) {
	return r.updatePin(ctx, videoID, authorUsername, map[string]any{"is_pinned": false, "pinned_at": nil})
}

func (r *PinRepository) updatePin(ctx context.Context, videoID uint, authorUsername string, changes map[string]any) (bool, error) {
	db := gormx.DB(ctx, r.db)
	result := db.
		Model(&po.Video{}).
		Where("id = ? AND author_username = ? AND status = ?", videoID, authorUsername, enum.VideoStatusPublished).
		Updates(changes)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	// MySQL 的零行结果也可能是幂等取消，需要用锁定读区分失效目标。
	var row po.Video
	err := db.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND author_username = ? AND status = ?", videoID, authorUsername, enum.VideoStatusPublished).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}
