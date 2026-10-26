// Package adapters 把提供方能力翻译成消费方拥有的跨上下文端口。
//
// 例如 video 用自己的 TranscodeTask 声明转码请求，适配器将它转换成 media 任务并调用媒体用例。
// 这样两端不直接导入对方的应用模型；字段名称、参数类型不同也不必强行共享一个大接口。
// 方法集已完全一致时可在 bootstrap 直接 wire.Bind，只有类型或调用形式不同才需要薄适配器。
// 这里负责字段映射与委派；新增业务决策应回到所属领域或用例，避免适配层形成第二套业务逻辑。
package adapters

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	chatapp "f_ccg_video_back_ddd/internal/application/chat"
	commentapp "f_ccg_video_back_ddd/internal/application/comment"
	dynamicapp "f_ccg_video_back_ddd/internal/application/dynamic"
	notificationapp "f_ccg_video_back_ddd/internal/application/notification"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
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
	_ commentapp.Notifier     = (*Notifier)(nil)
	_ dynamicapp.LikeNotifier = (*Notifier)(nil)
	_ videoapp.Notifier       = (*Notifier)(nil)
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

// NotifyDynamicLike 实现 dynamic.LikeNotifier：向动态作者发送点赞通知。
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

// NotifyVideoLike 实现 video.Notifier：视频被点赞通知。
func (a *Notifier) NotifyVideoLike(ctx context.Context, notice videoapp.Notice) {
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

// NotifySystemBatch 实现 video.Notifier：批量系统通知（举报处理结果）。
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
