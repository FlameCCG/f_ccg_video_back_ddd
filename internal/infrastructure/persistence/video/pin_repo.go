package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
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

// SetPinned 把作者名下指定视频标记为置顶，并写入置顶时间。
func (r *PinRepository) SetPinned(
	ctx context.Context,
	videoID uint,
	authorUsername string,
	pinnedAt *time.Time,
) error {
	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ? AND author_username = ?", videoID, authorUsername).
		Updates(map[string]any{"is_pinned": true, "pinned_at": pinnedAt}).Error
}

// ClearPinned 取消作者名下指定视频的置顶状态。
func (r *PinRepository) ClearPinned(ctx context.Context, videoID uint, authorUsername string) error {
	return gormx.DB(ctx, r.db).
		Model(&po.Video{}).
		Where("id = ? AND author_username = ?", videoID, authorUsername).
		Updates(map[string]any{"is_pinned": false, "pinned_at": nil}).Error
}
