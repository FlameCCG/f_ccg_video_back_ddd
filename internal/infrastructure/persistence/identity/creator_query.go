package identity

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/constants"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// CreatorVideoStatsQuery 创作中心「已发布视频统计汇总」的读侧投影实现。
//
// 该查询直接读 videos 表与视频统计缓存（与原
// service/user_service/creator_analytics_service.go 的 Overview 完全一致）。
// 按 CQRS 约定，读侧投影允许跨上下文表直读；video 上下文仍独占写侧。
type CreatorVideoStatsQuery struct {
	db    *gorm.DB
	redis *redis.Client
}

var _ app.CreatorVideoStatsQuery = (*CreatorVideoStatsQuery)(nil)

// NewCreatorVideoStatsQuery 构造创作中心视频统计查询。
func NewCreatorVideoStatsQuery(db *gorm.DB, redisClient *redis.Client) *CreatorVideoStatsQuery {
	return &CreatorVideoStatsQuery{db: db, redis: redisClient}
}

// creatorVideoStats 视频统计缓存值结构（与 video 上下文写入的 JSON 完全一致）。
type creatorVideoStats struct {
	Views         int64 `json:"views"`
	Likes         int64 `json:"likes"`
	CommentCount  int64 `json:"commentCount"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	DanmuCount    int64 `json:"danmuCount"`
}

// creatorVideoRow 视频统计行。
type creatorVideoRow struct {
	ID            uint
	Views         int64
	CommentCount  int64
	CoinCount     int64
	FavoriteCount int64
	DanmuCount    int64
}

// AggregateByAuthor 汇总作者所有已发布视频的统计（缓存值优先于库值）。
func (q *CreatorVideoStatsQuery) AggregateByAuthor(ctx context.Context, username string) (app.CreatorVideoAggregate, error) {
	var aggregate app.CreatorVideoAggregate

	if username == "" {
		return aggregate, nil
	}

	var videos []creatorVideoRow
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "views", "comment_count", "coin_count", "favorite_count", "danmu_count").
		Where("author_username = ? AND status = ?", username, enum.VideoStatusPublished).
		Find(&videos).Error; err != nil {
		return aggregate, err
	}

	cachedStats := q.fetchVideoStatsCache(ctx, videos)

	for _, video := range videos {
		stats := creatorVideoStats{
			Views:         video.Views,
			CommentCount:  video.CommentCount,
			CoinCount:     video.CoinCount,
			FavoriteCount: video.FavoriteCount,
			DanmuCount:    video.DanmuCount,
		}
		if cached, ok := cachedStats[video.ID]; ok && cached != nil {
			stats = *cached
		}

		aggregate.Views += stats.Views
		aggregate.Comments += stats.CommentCount
		aggregate.Coins += stats.CoinCount
		aggregate.Danmu += stats.DanmuCount
		aggregate.Favorites += stats.FavoriteCount
	}

	return aggregate, nil
}

func (q *CreatorVideoStatsQuery) fetchVideoStatsCache(
	ctx context.Context,
	videos []creatorVideoRow,
) map[uint]*creatorVideoStats {
	out := make(map[uint]*creatorVideoStats, len(videos))
	if q.redis == nil || len(videos) == 0 {
		return out
	}

	pipe := q.redis.Pipeline()

	cmds := make([]*redis.StringCmd, 0, len(videos))
	for _, video := range videos {
		cmds = append(cmds, pipe.Get(ctx, creatorVideoStatsCacheKey(video.ID)))
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return out
	}

	for idx, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			continue
		}

		var stats creatorVideoStats
		if err := json.Unmarshal(data, &stats); err != nil {
			continue
		}

		out[videos[idx].ID] = &stats
	}

	return out
}

func creatorVideoStatsCacheKey(videoID uint) string {
	return fmt.Sprintf("%s%d", constants.VideoDetailStatsKeyPrefix, videoID)
}

// FansCounter 粉丝数读侧投影实现（原 UserService.GetFansCount）。
//
// 关注关系的写侧归 social 上下文；此处仅为创作中心总览提供计数读模型。
type FansCounter struct {
	db *gorm.DB
}

var _ app.FansCounter = (*FansCounter)(nil)

// NewFansCounter 构造粉丝数查询。
func NewFansCounter(db *gorm.DB) *FansCounter {
	return &FansCounter{db: db}
}

// FansCount 统计用户粉丝数；查询失败返回 0（与原实现一致）。
func (c *FansCounter) FansCount(ctx context.Context, userID uint) int64 {
	if userID == 0 {
		return 0
	}

	var count int64
	if err := gormx.DB(ctx, c.db).
		Model(&po.UserFocus{}).
		Where("focus_user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0
	}

	return count
}
