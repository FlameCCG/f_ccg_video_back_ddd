package scheduler

import (
	commentapplication "f_ccg_video_back_ddd/internal/application/comment"
	identityapplication "f_ccg_video_back_ddd/internal/application/identity"
	platformapp "f_ccg_video_back_ddd/internal/application/platform"

	videoapp "f_ccg_video_back_ddd/internal/application/video"
)

var _ platformapp.UploadCleanupJob = (*videoapp.UploadCleanupService)(nil)

// 编译期断言：四个应用层用例服务分别满足对应的任务端口，
// bootstrap 中可直接 wire.Bind，无需额外适配器。
var (
	_ platformapp.CommentLikeSyncJob = (*commentapplication.CommentLikeSyncService)(nil)
	_ platformapp.VideoStatSyncJob   = (*videoapp.VideoStatSyncService)(nil)
	_ platformapp.HotRankDecayJob    = (*videoapp.HotRankDecayService)(nil)
	_ platformapp.AutoUnbanJob       = (*identityapplication.AutoUnbanService)(nil)
)
