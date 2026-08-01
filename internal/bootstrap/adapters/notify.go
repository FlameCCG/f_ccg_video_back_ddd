// Package adapters 存放跨限界上下文的端口适配器。
//
// 各上下文只在自己的 application 包里声明所需能力的**本地端口**，绝不 import 对方的包；
// 当某个端口的方法集与提供方的现有方法不能逐字匹配时（参数结构体不同、返回值不同等），
// 由本包提供一层极薄的转换适配器，并在 bootstrap 中用 wire.Bind 绑定。
//
// 这里只做字段搬运与类型转换，不含任何业务判断。
package adapters

import (
	"context"

	chatapp "f_ccg_video_back_ddd/internal/application/chat"
	commentapp "f_ccg_video_back_ddd/internal/application/comment"
	dynamicapp "f_ccg_video_back_ddd/internal/application/dynamic"
	notificationapp "f_ccg_video_back_ddd/internal/application/notification"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BadgeInvalidator 把通知角标缓存的 Invalidate 方法适配为 chat 的 InvalidateCount 端口。
//
// 它刻意只依赖缓存端口，避免 chat.Service 与 notification.Service 的构造环。
type BadgeInvalidator struct {
	cache notificationapp.CountCache
}

var _ chatapp.NotificationBadgeInvalidator = (*BadgeInvalidator)(nil)

// NewBadgeInvalidator 构造聊天写后角标失效适配器。
func NewBadgeInvalidator(cache notificationapp.CountCache) *BadgeInvalidator {
	return &BadgeInvalidator{cache: cache}
}

// InvalidateCount 失效指定接收者的角标缓存。
func (a *BadgeInvalidator) InvalidateCount(receiverID int) {
	a.cache.Invalidate(receiverID)
}

// Notifier 把 notification 应用服务适配为 comment / dynamic / video 三个上下文声明的通知端口。
type Notifier struct {
	notifications *notificationapp.Service
}

var (
	_ commentapp.Notifier            = (*Notifier)(nil)
	_ dynamicapp.DynamicLikeNotifier = (*Notifier)(nil)
	_ videoapp.VideoNotifier         = (*Notifier)(nil)
)

// NewNotifier 构造通知适配器。
func NewNotifier(notifications *notificationapp.Service) *Notifier {
	return &Notifier{notifications: notifications}
}

// CreateNotices 实现 comment.Notifier：批量写入通知（一次插入，失败则整批不生效）。
func (a *Notifier) CreateNotices(ctx context.Context, notices []commentapp.Notice) error {
	if len(notices) == 0 {
		return nil
	}

	rows := make([]notificationdomain.Notification, 0, len(notices))
	for _, n := range notices {
		rows = append(rows, notificationdomain.Notification{
			Type:             n.Type,
			ReceiverID:       n.ReceiverID,
			ActionUserID:     n.ActionUserID,
			ActionUserAvatar: n.ActionUserAvatar,
			ActionUserName:   n.ActionUserName,
			Title:            n.Title,
			Content:          n.Content,
			Link:             n.Link,
			VideoID:          n.VideoID,
			VideoTitle:       n.VideoTitle,
			CommentID:        n.CommentID,
		})
	}

	return a.notifications.CreateBatch(ctx, rows)
}

// InvalidateCount 实现 comment.Notifier / chat.NotificationBadgeInvalidator：失效角标计数缓存。
func (a *Notifier) InvalidateCount(receiverID int) {
	a.notifications.InvalidateCount(receiverID)
}

// NotifyDynamicLike 实现 dynamic.DynamicLikeNotifier：向动态作者发送点赞通知。
func (a *Notifier) NotifyDynamicLike(
	ctx context.Context,
	receiverID uint,
	dynamicID uint,
	actionUserID uint,
	actionUserName, actionUserAvatar, title, content string,
) error {
	return a.notifications.Create(ctx, &notificationdomain.Notification{
		Type:             enum.NotificationLikeDynamicType,
		ReceiverID:       int(receiverID),
		DynamicID:        dynamicID,
		ActionUserID:     actionUserID,
		ActionUserName:   actionUserName,
		ActionUserAvatar: actionUserAvatar,
		Title:            title,
		Content:          content,
	})
}

// NotifyVideoLike 实现 video.VideoNotifier：视频被点赞通知。
func (a *Notifier) NotifyVideoLike(ctx context.Context, notice videoapp.VideoNotice) {
	_ = a.notifications.Create(ctx, &notificationdomain.Notification{
		Type:             enum.NotificationVideoLikeType,
		ReceiverID:       int(notice.ReceiverID),
		ActionUserID:     notice.ActionUserID,
		ActionUserName:   notice.ActionUserName,
		ActionUserAvatar: notice.ActionUserAvatar,
		Title:            notice.Title,
		Content:          notice.Content,
		VideoID:          notice.VideoID,
		VideoTitle:       notice.VideoTitle,
	})
}

// NotifySystemBatch 实现 video.VideoNotifier：批量系统通知（举报处理结果）。
func (a *Notifier) NotifySystemBatch(ctx context.Context, notices []videoapp.SystemNotice) {
	if len(notices) == 0 {
		return
	}

	rows := make([]notificationdomain.Notification, 0, len(notices))
	for _, n := range notices {
		rows = append(rows, notificationdomain.Notification{
			Type:             enum.NotificationSystemType,
			ReceiverID:       int(n.ReceiverID),
			ActionUserID:     n.ActionUserID,
			ActionUserName:   n.ActionUserName,
			ActionUserAvatar: n.ActionUserAvatar,
			Title:            n.Title,
			Content:          n.Content,
		})
	}

	_ = a.notifications.CreateBatch(ctx, rows)
}
