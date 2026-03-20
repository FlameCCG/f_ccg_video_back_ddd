package video

import (
	"context"

	videovo "f_ccg_video_back_ddd/internal/domain/video/valueobject"
)

// StatsCache 视频统计的 Redis 权威计数端口。
//
// Redis key、TTL、Lua 脚本语义、dirty 集合与 version key 均属于外部契约，由 infrastructure 适配器实现。
type StatsCache interface {
	// Available 报告 Redis 是否可用；不可用时应用层直接写 DB。
	Available() bool
	// Load 读取权威计数；未命中返回 (nil, false)。
	Load(ctx context.Context, videoID uint) (*videovo.Stats, bool)
	// LoadBatch 批量读取权威计数（MGET），缺失项不出现在结果里。
	LoadBatch(ctx context.Context, videoIDs []uint) map[uint]*videovo.Stats
	// Seed 仅在缺失时回填计数（SETNX + TTL）。
	Seed(ctx context.Context, videoID uint, stats *videovo.Stats)
	// ApplyDelta 原子应用增量并标记 dirty + 递增 version；base 为冷缓存时的基线 payload。
	ApplyDelta(ctx context.Context, videoID uint, delta videovo.StatsDelta, basePayload string) (*videovo.Stats, error)
	// ApplyDeltaIfExists 仅在 key 已存在时应用增量；key 不存在返回 (nil, false, nil)。
	ApplyDeltaIfExists(ctx context.Context, videoID uint, delta videovo.StatsDelta) (*videovo.Stats, bool, error)
	// RawPayload 读取当前 stats key 的原始字符串；不存在返回 ("", false, nil)。
	RawPayload(ctx context.Context, videoID uint) (string, bool, error)
	// Clear 删除 stats / version / legacy synced key 并移出 dirty 集合。
	Clear(ctx context.Context, videoIDs ...uint)
	// IsTransientError 判断错误是否为 Redis 瞬时错误（可重试）。
	IsTransientError(err error) bool
}

// AuthorIDCache 视频作者 ID 的 Redis 缓存端口。
type AuthorIDCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Load 读取作者 ID；未命中返回 (0, false)。
	Load(ctx context.Context, videoID uint) (uint, bool)
	// Store 写入作者 ID（长 TTL）。
	Store(ctx context.Context, videoID, authorID uint)
	// Invalidate 删除作者 ID 缓存。
	Invalidate(ctx context.Context, videoID uint)
}

// AuthorStatsCache 作者作品聚合统计的进程内短 TTL 缓存端口。
type AuthorStatsCache interface {
	Load(authorUsername string) (UserVideoStats, bool)
	Store(authorUsername string, stats UserVideoStats)
}

// CreatorAnalytics 创作中心指标累加端口。
//
// 绑定目标：identity 上下文的 *identity.CreatorAnalyticsService。
type CreatorAnalytics interface {
	IncrViews(ctx context.Context, creatorID uint, delta int64) error
	IncrComments(ctx context.Context, creatorID uint, delta int64) error
	IncrCoins(ctx context.Context, creatorID uint, delta int64) error
	IncrDanmu(ctx context.Context, creatorID uint, delta int64) error
	IncrFavorites(ctx context.Context, creatorID uint, delta int64) error
}
