package scheduler

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/platform"
)

// CommentLikeSyncJob 评论点赞增量落库任务端口。
//
// 绑定到 application/platform.CommentLikeSyncService。
type CommentLikeSyncJob interface {
	// Run 执行一轮同步。
	Run(ctx context.Context)
}

// VideoStatSyncJob 视频统计缓存回写任务端口。
//
// 绑定到 application/platform.VideoStatSyncService。
type VideoStatSyncJob interface {
	// Run 执行一轮同步。
	Run(ctx context.Context)
}

// HotRankDecayJob 热门榜时间衰减任务端口。
//
// 绑定到 application/platform.HotRankDecayService。
type HotRankDecayJob interface {
	// Run 执行一轮衰减。
	Run(ctx context.Context)
}

// AutoUnbanJob 到期用户自动解封任务端口。
//
// 绑定到 application/platform.AutoUnbanService。
type AutoUnbanJob interface {
	// Run 执行一轮解封检查。
	Run(ctx context.Context)
}

// 编译期断言：四个应用层用例服务分别满足对应的任务端口，
// bootstrap 中可直接 wire.Bind，无需额外适配器。
var (
	_ CommentLikeSyncJob = (*app.CommentLikeSyncService)(nil)
	_ VideoStatSyncJob   = (*app.VideoStatSyncService)(nil)
	_ HotRankDecayJob    = (*app.HotRankDecayService)(nil)
	_ AutoUnbanJob       = (*app.AutoUnbanService)(nil)
)
