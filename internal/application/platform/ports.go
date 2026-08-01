package platform

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/platform"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// UserStatusWriter 用户状态缓存写穿端口（跨上下文）。
//
// 由 identity 上下文的应用服务在 bootstrap 中绑定实现
// （*application/identity.UserStatusService 的 Write 方法），
// 自动解封成功后 write-through 写入 Active，避免仅 DEL 被在途旧 Banned 回填。
type UserStatusWriter interface {
	// Write 权威写穿用户状态缓存。
	Write(userID uint, status enum.UserStatus)
}

// BanMaintenanceRepository 封禁维护仓储端口：到期封禁的查询与批量解封落库。
//
// 按 D-011 读写投影约定，实现直连 users / user_ban_records 表，
// SQL 语义与原 cron_service.AutoUnbanUsers 逐字一致。
type BanMaintenanceRepository interface {
	// ListExpiredBans 查询每个用户最新一条封禁记录中已到期、且用户仍处于封禁态的条目。
	ListExpiredBans(ctx context.Context, now time.Time) ([]domain.ExpiredBan, error)
	// ActivateBannedUsers 将仍处于封禁态的指定用户批量置为正常状态。
	ActivateBannedUsers(ctx context.Context, userIDs []uint) error
	// MarkBanRecordsUnbanned 批量标记原封禁记录已解封。
	MarkBanRecordsUnbanned(ctx context.Context, recordIDs []uint) error
}

// HotRankStore 热门榜单存储端口（Redis 有序集合）。
type HotRankStore interface {
	// Available 报告底层 Redis 是否已配置。
	Available() bool
	// Size 返回榜单当前成员数。
	Size(ctx context.Context) (int64, error)
	// Entries 返回榜单全部成员及分值，按分值升序（与 ZRANGE 0 -1 一致）。
	Entries(ctx context.Context) ([]domain.HotRankEntry, error)
	// ApplyDecay 按入参顺序在同一管线内写回新分值或移出成员。
	ApplyDecay(ctx context.Context, updates []domain.HotRankUpdate) error
	// TrimLowest 移除分值最低的 count 个成员。
	TrimLowest(ctx context.Context, count int64) error
}

// VideoStatsCache 视频统计缓存端口：脏集合游标扫描、快照原子读取、版本确认。
type VideoStatsCache interface {
	// Available 报告底层 Redis 是否已配置。
	Available() bool
	// ScanDirty 游标扫描脏视频集合，返回本批成员与下一个游标。
	ScanDirty(ctx context.Context, cursor uint64, count int64) ([]string, uint64, error)
	// LoadSnapshots 原子读取每个成员的统计值与版本号；
	// 成员非法或统计值缺失时由实现直接摘除脏标记并跳过。
	LoadSnapshots(ctx context.Context, ids []string) ([]VideoStatsSnapshot, error)
	// AckVersion 版本未变化时摘除脏标记；版本已变化则保留，等待下轮同步。
	AckVersion(ctx context.Context, memberID string, version int64) error
}

// VideoStatsProjection 视频统计写侧投影端口（直连 videos 表）。
type VideoStatsProjection interface {
	// PersistStats 将统计快照回写到已发布视频行。
	PersistStats(ctx context.Context, videoID uint, stats VideoStats) error
}

// CommentLikeQueue 评论点赞增量队列端口（Redis 列表 + processing 中转队列）。
type CommentLikeQueue interface {
	// Available 报告底层 Redis 是否已配置。
	Available() bool
	// NextBatch 取下一批待处理原始消息：
	// processing 队列有残留时优先重放，否则从源队列原子搬运一批。
	NextBatch(ctx context.Context) ([]string, error)
	// Ack 确认 processing 队列头部 count 条消息已处理完毕。
	Ack(ctx context.Context, count int) error
}

// CommentLikeProjection 评论点赞写侧投影端口（直连 comments / comment_likes 表）。
type CommentLikeProjection interface {
	// CommentExists 报告评论是否仍存在。
	CommentExists(ctx context.Context, commentID uint) (bool, error)
	// InsertLike 插入点赞行；已存在（唯一键冲突）时返回 false。
	InsertLike(ctx context.Context, commentID, userID uint) (bool, error)
	// DeleteLike 删除点赞行；本无该行时返回 false。
	DeleteLike(ctx context.Context, commentID, userID uint) (bool, error)
	// IncrLikeCount 评论点赞数 +1。
	IncrLikeCount(ctx context.Context, commentID uint) error
	// DecrLikeCount 评论点赞数 -1，下限为 0。
	DecrLikeCount(ctx context.Context, commentID uint) error
}
