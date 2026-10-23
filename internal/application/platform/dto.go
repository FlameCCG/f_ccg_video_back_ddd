// Package platform 编排平台周期维护任务的用例编排。
//
// 四条主线是评论点赞增量落库、视频统计回写、热门榜衰减和到期解封；触发时间由 infrastructure/scheduler 决定。
// 领域计算折叠或衰减结果，应用层安排读取、写入与确认。理解重试时应逐项查看队列确认、版本和失败恢复策略。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
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
