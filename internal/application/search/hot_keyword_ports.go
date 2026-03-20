package search

import (
	"context"
)

// HotVideoTitle 是热词回退采样得到的视频标题与播放量.
type HotVideoTitle struct {
	Title string
	Views int64
}

// HotKeywordFallbackQuery Redis 热搜词榜为空时的回退查询端口.
type HotKeywordFallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// HotVideoTitles 按播放量取热门已发布视频标题样本。
	HotVideoTitles(ctx context.Context, sampleSize int) ([]HotVideoTitle, error)
}

// HotKeywordEntry 是 Redis 热搜词榜的一个成员及其分数.
type HotKeywordEntry struct {
	Member string
	Score  float64
}

// HotKeywordStore 热搜词榜存储端口。
//
// Redis key（search:hot:keywords:zset）与「只增不过期」的语义属于外部契约。
type HotKeywordStore interface {
	// Available 报告 Redis 客户端是否就绪。
	Available() bool
	// Card 返回榜单成员总数。
	Card(ctx context.Context) (int64, error)
	// RevRangeWithScores 按分数倒序取 [start,stop] 闭区间成员。
	RevRangeWithScores(ctx context.Context, start, stop int64) ([]HotKeywordEntry, error)
	// Incr 给关键词热度加一；失败静默忽略（best-effort）。
	Incr(ctx context.Context, keyword string)
}
