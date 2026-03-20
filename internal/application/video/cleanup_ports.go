package video

import (
	"context"
)

// TranscodeCanceller 转码取消通知端口。
//
// 绑定目标：media 上下文。
type TranscodeCanceller interface {
	// NotifyVideoDeleted 广播视频已删除事件，取消在途转码任务。
	NotifyVideoDeleted(ctx context.Context, videoIDs []uint) error
}

// CommentPurger 视频关联评论清理端口。
//
// 绑定目标：comment 上下文。
// 作者软删路径在事务内调用，comment 侧实现会复用 context 中的事务。
type CommentPurger interface {
	// PurgeByVideoIDs 删除给定视频的全部评论与评论点赞，返回删除计数。
	PurgeByVideoIDs(ctx context.Context, videoIDs []uint) (comments, commentLikes int64, err error)
}
