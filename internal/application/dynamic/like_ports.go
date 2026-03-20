package dynamic

import (
	"context"
)

// DynamicLikeNotifier 动态点赞通知端口（由 notification 上下文实现）。
type DynamicLikeNotifier interface {
	// NotifyDynamicLike 向动态作者发送「有人点赞了你的动态」通知。
	NotifyDynamicLike(
		ctx context.Context,
		receiverID uint,
		dynamicID uint,
		actionUserID uint,
		actionUserName, actionUserAvatar, title, content string,
	) error
}
