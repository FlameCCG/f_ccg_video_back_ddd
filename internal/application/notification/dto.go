// Package notification 是通知上下文的应用层：用例服务、DTO 与端口。
package notification

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// NotificationItem 通知列表项（读模型，json tag 与原实现一致）.
type NotificationItem struct {
	ID               uint      `json:"id"`
	Type             enum.Type `json:"type"`
	ReceiverID       int       `json:"receiverID"`
	ActionUserID     uint      `json:"actionUserID"`
	ActionUserAvatar string    `json:"actionUserAvatar"`
	ActionUserName   string    `json:"actionUserName"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	VideoID          uint      `json:"videoID"`
	VideoTitle       string    `json:"videoTitle"`
	Link             string    `json:"link"`
	CommentID        uint      `json:"commentID"`
	DynamicID        uint      `json:"dynamicID"`
	IsRead           bool      `json:"isRead"`
	CreatedAt        time.Time `json:"createdAt"`
}

// NotificationCountSummary 角标计数汇总（读模型）.
type NotificationCountSummary struct {
	Reply   int64 `json:"reply"`
	Like    int64 `json:"like"`
	At      int64 `json:"at"`
	System  int64 `json:"system"`
	Message int64 `json:"message"`
}

// CountCache 角标计数缓存端口（Redis，fresh / stale-while-revalidate）。
type CountCache interface {
	// Available 报告底层缓存是否可用。
	Available() bool
	// Load 读取缓存；返回值分别为数据、是否新鲜、是否命中。
	Load(receiverID int) (*NotificationCountSummary, bool, bool)
	// Store 写入缓存。
	Store(receiverID int, data NotificationCountSummary)
	// Invalidate 删除指定接收者的计数缓存。
	Invalidate(receiverID int)
	// InvalidateAll 清除全部计数缓存（全员广播场景）。
	InvalidateAll()
}

// ChatUnreadCounter 聊天未读数查询端口。
//
// 由 chat 上下文的应用服务在 bootstrap 中绑定实现，实现跨上下文协调。
type ChatUnreadCounter interface {
	TotalUnread(ctx context.Context, userID uint) (int64, error)
}
