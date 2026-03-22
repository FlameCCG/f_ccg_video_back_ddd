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

// Notification 通知实体，承载纯内容，不含已读状态。
//
// 状态不对外暴露，管理端 JSON 由应用层响应 DTO 输出。
type Notification struct {
	id               uint
	createdAt        time.Time
	updatedAt        time.Time
	noticeType       enum.Type
	receiverID       int
	actionUserID     uint
	actionUserAvatar string
	actionUserName   string
	title            string
	content          string
	link             string
	videoID          uint
	videoTitle       string
	dynamicID        uint
	commentID        uint
}

// Draft 是创建通知的输入。不包含身份与时间，字段保持公开。
type Draft struct {
	Type             enum.Type
	ReceiverID       int
	ActionUserID     uint
	ActionUserAvatar string
	ActionUserName   string
	Title            string
	Content          string
	Link             string
	VideoID          uint
	VideoTitle       string
	DynamicID        uint
	CommentID        uint
}

// Snapshot 是持久化快照。字段公开，供仓储重建历史数据。
type Snapshot struct {
	ID               uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Type             enum.Type
	ReceiverID       int
	ActionUserID     uint
	ActionUserAvatar string
	ActionUserName   string
	Title            string
	Content          string
	Link             string
	VideoID          uint
	VideoTitle       string
	DynamicID        uint
	CommentID        uint
}

// SystemContent 是系统通知允许编辑的内容，不包含操作者身份及创建时间。
type SystemContent struct {
	ReceiverID           int
	Title, Content, Link string
	VideoID              uint
	VideoTitle           string
}

// NewNotification 按调用方给出的内容构造通知，不附加新的业务限制，也不分配身份或时间。
func NewNotification(draft Draft) *Notification {
	return &Notification{
		noticeType:       draft.Type,
		receiverID:       draft.ReceiverID,
		actionUserID:     draft.ActionUserID,
		actionUserAvatar: draft.ActionUserAvatar,
		actionUserName:   draft.ActionUserName,
		title:            draft.Title,
		content:          draft.Content,
		link:             draft.Link,
		videoID:          draft.VideoID,
		videoTitle:       draft.VideoTitle,
		dynamicID:        draft.DynamicID,
		commentID:        draft.CommentID,
	}
}

// RestoreNotification 从持久化快照恢复历史通知，不重新执行创建校验。
func RestoreNotification(snapshot Snapshot) *Notification {
	return &Notification{
		id:               snapshot.ID,
		createdAt:        snapshot.CreatedAt,
		updatedAt:        snapshot.UpdatedAt,
		noticeType:       snapshot.Type,
		receiverID:       snapshot.ReceiverID,
		actionUserID:     snapshot.ActionUserID,
		actionUserAvatar: snapshot.ActionUserAvatar,
		actionUserName:   snapshot.ActionUserName,
		title:            snapshot.Title,
		content:          snapshot.Content,
		link:             snapshot.Link,
		videoID:          snapshot.VideoID,
		videoTitle:       snapshot.VideoTitle,
		dynamicID:        snapshot.DynamicID,
		commentID:        snapshot.CommentID,
	}
}

func (n *Notification) ID() uint { return n.id }

func (n *Notification) CreatedAt() time.Time { return n.createdAt }

func (n *Notification) UpdatedAt() time.Time { return n.updatedAt }

func (n *Notification) Type() enum.Type { return n.noticeType }

func (n *Notification) ReceiverID() int { return n.receiverID }

func (n *Notification) ActionUserID() uint { return n.actionUserID }

func (n *Notification) ActionUserAvatar() string { return n.actionUserAvatar }

func (n *Notification) ActionUserName() string { return n.actionUserName }

func (n *Notification) Title() string { return n.title }

func (n *Notification) Content() string { return n.content }

func (n *Notification) Link() string { return n.link }

func (n *Notification) VideoID() uint { return n.videoID }

func (n *Notification) VideoTitle() string { return n.videoTitle }

func (n *Notification) DynamicID() uint { return n.dynamicID }

func (n *Notification) CommentID() uint { return n.commentID }

// Snapshot 返回当前状态的副本。通知没有切片、映射或指针字段，值复制即可隔离开调用方。
func (n *Notification) Snapshot() Snapshot {
	return Snapshot{
		ID:               n.id,
		CreatedAt:        n.createdAt,
		UpdatedAt:        n.updatedAt,
		Type:             n.noticeType,
		ReceiverID:       n.receiverID,
		ActionUserID:     n.actionUserID,
		ActionUserAvatar: n.actionUserAvatar,
		ActionUserName:   n.actionUserName,
		Title:            n.title,
		Content:          n.content,
		Link:             n.link,
		VideoID:          n.videoID,
		VideoTitle:       n.videoTitle,
		DynamicID:        n.dynamicID,
		CommentID:        n.commentID,
	}
}

// ReviseAsSystem 将通知转换为系统通知，同时更新类型与可编辑字段；身份、操作者与创建时间不变。
func (n *Notification) ReviseAsSystem(change SystemContent) {
	n.noticeType = enum.NotificationSystemType
	n.receiverID = change.ReceiverID
	n.title = change.Title
	n.content = change.Content
	n.link = change.Link
	n.videoID = change.VideoID
	n.videoTitle = change.VideoTitle
}

// EnsureDeletable 拒绝系统消息和全员广播。失败时不修改通知。
// 其它接收者的普通通知不视为错误，是否纳入删除由 DeletableBy 决定。
func (n *Notification) EnsureDeletable() error {
	if n == nil || n.undeletable() {
		return ErrSystemNotificationNotDeletable
	}

	return nil
}

// DeletableBy 报告该接收者能否删除这条通知。
func (n *Notification) DeletableBy(receiverID int) bool {
	if n == nil || receiverID <= 0 || n.undeletable() {
		return false
	}

	return n.receiverID == receiverID
}

func (n *Notification) undeletable() bool {
	return n.noticeType == enum.NotificationSystemType || n.receiverID == BroadcastReceiverID
}
