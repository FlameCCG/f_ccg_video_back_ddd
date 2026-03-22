package scheduler

import (
	"context"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// VideoStatsProjection 视频统计写侧投影的 GORM 实现（直连 videos 表）。
type VideoStatsProjection struct {
	db *gorm.DB
}

var _ videodomain.VideoStatsProjection = (*VideoStatsProjection)(nil)

// NewVideoStatsProjection 构造视频统计投影仓储。
func NewVideoStatsProjection(db *gorm.DB) *VideoStatsProjection {
	return &VideoStatsProjection{db: db}
}

// PersistStats 仅将统计快照回写到已发布的视频行。
func (p *VideoStatsProjection) PersistStats(ctx context.Context, videoID uint, stats videodomain.Stats) error {
	return gormx.DB(ctx, p.db).
		Model(&po.Video{}).
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		Updates(map[string]any{
			"views":          stats.Views,
			"likes":          stats.Likes,
			"comment_count":  stats.CommentCount,
			"coin_count":     stats.CoinCount,
			"favorite_count": stats.FavoriteCount,
			"danmu_count":    stats.DanmuCount,
		}).Error
}
