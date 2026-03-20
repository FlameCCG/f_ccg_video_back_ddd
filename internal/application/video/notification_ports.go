package video

import (
	"context"
)

// NotifySourceQuery 通知渲染所需的读侧投影端口。
//
// 这些只读投影（users / videos 列）允许在 persistence/video 直读，
// 不必经跨上下文端口；写入通知行仍走 VideoNotifier。
type NotifySourceQuery interface {
	// ActionUser 读取用户 id/username/avatar；不存在时返回 (零值, false)。
	ActionUser(ctx context.Context, userID uint) (ActionUser, bool)
	// VideoAuthorAndTitle 读取视频标题与作者用户 ID；任一缺失时返回 ok=false。
	VideoAuthorAndTitle(ctx context.Context, videoID uint) (authorID uint, videoTitle string, ok bool)
	// UserIDsByUsernames 按用户名批量解析用户 ID。
	UserIDsByUsernames(ctx context.Context, usernames []string) (map[string]uint, error)
}

// UsernameCache 用户 ID → 用户名的进程内缓存端口。
type UsernameCache interface {
	Load(userID uint) (string, bool)
	Store(userID uint, username string)
	Delete(userID uint)
}

// VideoNotifier 通知端口。
//
// 绑定目标：notification 上下文的 DeliveryService；角标失效由 CountService 提供。
// 写入通知行后失效对应接收者的角标缓存；失败仅记录日志。
type VideoNotifier interface {
	// NotifyVideoLike 视频被点赞通知（类型固定为 enum.NotificationVideoLikeType）。
	NotifyVideoLike(ctx context.Context, notice VideoNotice)
	// NotifySystemBatch 批量系统通知（类型固定为 enum.NotificationSystemType，举报处理结果）。
	NotifySystemBatch(ctx context.Context, notices []SystemNotice)
}

// ActionUser 触发通知的操作用户信息。
type ActionUser struct {
	ID       uint
	Username string
	Avatar   string
}

// VideoNotice 一条视频相关通知（点赞）。
type VideoNotice struct {
	ReceiverID       uint
	ActionUserID     uint
	ActionUserName   string
	ActionUserAvatar string
	Title            string
	Content          string
	VideoID          uint
	VideoTitle       string
}

// SystemNotice 一条系统通知。
type SystemNotice struct {
	ReceiverID       uint
	ActionUserID     uint
	ActionUserName   string
	ActionUserAvatar string
	Title            string
	Content          string
}
