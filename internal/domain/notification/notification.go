// Package notification 是通知上下文的领域层：通知实体、分类规则与仓储接口。
package notification

import (
	"errors"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BroadcastReceiverID 全员广播通知的接收者标识（-1）。
const BroadcastReceiverID = -1

// ErrSystemNotificationNotDeletable 系统消息不允许用户删除.
var ErrSystemNotificationNotDeletable = errors.New("系统消息不可删除")

// Notification 通知实体（纯内容，不含已读状态）。
//
// json tag 与原 models/mysqlEntity.UserGlobalNotification 完全一致，
// 管理端接口直接序列化该结构，保证响应 JSON 1:1。
type Notification struct {
	ID               uint      `json:"id"`
	CreatedAt        time.Time `json:"-"`
	UpdatedAt        time.Time `json:"-"`
	Type             enum.Type `json:"type"`
	ReceiverID       int       `json:"receiverID"`
	ActionUserID     uint      `json:"actionUserID"`
	ActionUserAvatar string    `json:"actionUserAvatar"`
	ActionUserName   string    `json:"actionUserName"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	Link             string    `json:"link"`
	VideoID          uint      `json:"videoID"`
	VideoTitle       string    `json:"videoTitle"`
	DynamicID        uint      `json:"dynamicID"`
	CommentID        uint      `json:"commentID"`
}

// AllTypes 返回参与角标统计与列表展示的全部通知类型（顺序与原实现一致）。
func AllTypes() []enum.Type {
	return []enum.Type{
		enum.NotificationApplyType,
		enum.NotificationVideoLikeType,
		enum.NotificationLikeCommentType,
		enum.NotificationLikeDynamicType,
		enum.NotificationAtType,
		enum.NotificationSystemType,
	}
}

// TypesByCategory 把前端分类映射为通知类型集合（与原 typesByCategory 完全一致）。
func TypesByCategory(category string) []enum.Type {
	switch category {
	case "reply":
		return []enum.Type{enum.NotificationApplyType}
	case "like":
		return []enum.Type{
			enum.NotificationVideoLikeType,
			enum.NotificationLikeCommentType,
			enum.NotificationLikeDynamicType,
		}
	case "at":
		return []enum.Type{enum.NotificationAtType}
	case "system":
		return []enum.Type{enum.NotificationSystemType}
	default:
		return AllTypes()
	}
}
