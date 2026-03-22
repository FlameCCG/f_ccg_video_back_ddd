package video

import (
	"context"
	videodomain "f_ccg_video_back_ddd/internal/domain/video" // HotRankStore 热门榜单存储端口（Redis 有序集合）。
)

type HotRankStore interface {
	// Available 报告底层 Redis 是否已配置。
	Available() bool
	// Size 返回榜单当前成员数。
	Size(ctx context.Context) (int64, error)
	// Entries 返回榜单全部成员及分值，按分值升序（与 ZRANGE 0 -1 一致）。
	Entries(ctx context.Context) ([]videodomain.HotRankEntry, error)
	// ApplyDecay 按入参顺序在同一管线内写回新分值或移出成员。
	ApplyDecay(ctx context.Context, updates []videodomain.HotRankUpdate) error
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
