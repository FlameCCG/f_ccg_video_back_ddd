package video

import (
	"context"
	"fmt"
	"time"

	videorepo "f_ccg_video_back_ddd/internal/domain/video/repository"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// maxHistoryWindow 首页去重历史窗口。
const maxHistoryWindow = 100

// HistoryRepository 播放历史仓储的 GORM 实现。
type HistoryRepository struct {
	db *gorm.DB
}

var _ videorepo.HistoryRepository = (*HistoryRepository)(nil)

// NewHistoryRepository 构造播放历史仓储。
func NewHistoryRepository(db *gorm.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

// Upsert 通过 GORM clause.OnConflict 写入或更新播放进度。
//
// 主键冲突时覆盖 progress、duration 和 created_at；支持 MySQL 与 SQLite。
func (r *HistoryRepository) Upsert(ctx context.Context, userID, videoID uint, progress, duration int) error {
	now := time.Now()
	row := po.VideoPlayHistory{
		UserID:    userID,
		VideoID:   videoID,
		Progress:  progress,
		Duration:  duration,
		CreatedAt: now,
	}

	return gormx.DB(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"progress":   progress,
				"duration":   duration,
				"created_at": now,
			}),
		}).
		Create(&row).Error
}

// Find 读取播放进度；不存在时返回 ErrRecordNotFound。
func (r *HistoryRepository) Find(ctx context.Context, userID, videoID uint) (*videorepo.PlayHistory, error) {
	var row po.VideoPlayHistory
	if err := gormx.DB(ctx, r.db).
		Where("user_id = ? AND video_id = ?", userID, videoID).
		Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}

	return toDomainPlayHistory(&row), nil
}

// DeleteByVideoIDs 批量删除用户的历史记录。
func (r *HistoryRepository) DeleteByVideoIDs(ctx context.Context, userID uint, videoIDs []uint) error {
	if len(videoIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Where("user_id = ? AND video_id IN ?", userID, videoIDs).
		Delete(&po.VideoPlayHistory{}).Error
}

// RecentVideoIDs 读取用户最近观看的视频 ID（按时间倒序，最多 limit 条）。
func (r *HistoryRepository) RecentVideoIDs(ctx context.Context, userID uint, limit int) ([]uint, error) {
	if userID == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = maxHistoryWindow
	}

	var videoIDs []uint

	if err := gormx.DB(ctx, r.db).
		Model(&po.VideoPlayHistory{}).
		Select("video_id").
		Where("user_id = ?", userID).
		Limit(limit).
		Order("created_at DESC").
		Pluck("video_id", &videoIDs).Error; err != nil {
		return nil, fmt.Errorf("querying feed history: %w", err)
	}

	return videoIDs, nil
}

// ProgressByVideoIDs 批量读取用户在给定视频上的进度（仅返回 >0 的项）。
func (r *HistoryRepository) ProgressByVideoIDs(ctx context.Context, userID uint, videoIDs []uint) map[uint]int {
	if userID == 0 || len(videoIDs) == 0 {
		return nil
	}

	type row struct {
		VideoID  uint
		Progress int
	}

	var rows []row

	_ = gormx.DB(ctx, r.db).
		Model(&po.VideoPlayHistory{}).
		Select("video_id, progress").
		Where("user_id = ? AND video_id IN ?", userID, videoIDs).
		Find(&rows).Error

	if len(rows) == 0 {
		return nil
	}

	out := make(map[uint]int, len(rows))
	for _, item := range rows {
		if item.Progress > 0 {
			out[item.VideoID] = item.Progress
		}
	}

	return out
}
