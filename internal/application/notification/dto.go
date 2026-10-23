// Package notification 编排通知列表、已读状态与角标计数。
//
// notification_service.go 将通知仓储、操作者资料、私信未读数与计数缓存组合为面向页面的结果。
// 通知实体不承载用户资料查询；资料和跨上下文计数通过消费方端口取得，写入后再处理角标缓存失效。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
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

// ActorProfileProvider 操作者资料查询端口（读侧投影，实现在 infrastructure）。
//
// 管理端发布系统通知时需要把操作者的用户名与头像写进通知快照字段。
type ActorProfileProvider interface {
	// Profile 返回操作者的用户名与头像；查不到时返回两个空串。
	Profile(ctx context.Context, userID uint) (username, avatar string)
}

// CreateSystemNoticeCommand 管理端发布系统通知的入参（字段与原请求体一一对应）。
type CreateSystemNoticeCommand struct {
	ActorID    uint
	Title      string
	Content    string
	ReceiverID int
	VideoID    uint
	VideoTitle string
	Link       string
}

// UpdateSystemNoticeCommand 管理端更新系统通知的入参。
type UpdateSystemNoticeCommand struct {
	ID         uint
	Title      string
	Content    string
	ReceiverID int
	VideoID    uint
	VideoTitle string
	Link       string
}
