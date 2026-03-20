package entity

import (
	"errors"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BroadcastReceiverID 全员广播通知的接收者标识（-1）。
const BroadcastReceiverID = -1

// ErrSystemNotificationNotDeletable 系统消息不允许用户删除.
var ErrSystemNotificationNotDeletable = errors.New("系统消息不可删除")

// Notification 通知实体，承载纯内容，不含已读状态。
//
// 管理端接口直接序列化此结构，JSON 字段属于接口契约。
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

// SystemContent 是系统通知允许编辑的内容，不包含操作者身份及创建时间。
type SystemContent struct {
	ReceiverID           int
	Title, Content, Link string
	VideoID              uint
	VideoTitle           string
}

// ReviseAsSystem 将通知转换为系统通知，同时更新类型与可编辑字段。
func (n *Notification) ReviseAsSystem(change SystemContent) {
	n.Type = enum.NotificationSystemType
	n.ReceiverID = change.ReceiverID
	n.Title, n.Content, n.Link = change.Title, change.Content, change.Link
	n.VideoID, n.VideoTitle = change.VideoID, change.VideoTitle
}

// AllTypes 返回参与角标统计与列表展示的全部通知类型。
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

// TypesByCategory 把前端分类映射为通知类型集合。
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
