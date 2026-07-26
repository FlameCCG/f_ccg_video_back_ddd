// Package platform 是平台运维上下文的应用层：定时任务用例编排与其依赖端口。
//
// 四个用例与原 service/cron_service 的四个定时任务一一对应：
// 评论点赞增量落库、视频统计缓存回写、热门榜时间衰减、到期用户自动解封。
package platform

// VideoStats 视频统计快照值：Redis 缓存中的六个计数字段。
type VideoStats struct {
	// Views 播放量。
	Views int64
	// Likes 点赞量。
	Likes int64
	// CommentCount 评论量。
	CommentCount int64
	// CoinCount 投币量。
	CoinCount int64
	// FavoriteCount 收藏量。
	FavoriteCount int64
	// DanmuCount 弹幕量。
	DanmuCount int64
}

// VideoStatsSnapshot 一个脏视频的统计快照：统计值与其版本号原子读取自同一时刻。
type VideoStatsSnapshot struct {
	// VideoID 视频 ID。
	VideoID uint
	// Version 读取快照时的版本号，用于确认后 CAS 摘除脏标记。
	Version int64
	// Stats 统计值；为 nil 表示该成员无需回写。
	Stats *VideoStats
	// MemberID 脏集合中的原始成员字符串。
	MemberID string
}
