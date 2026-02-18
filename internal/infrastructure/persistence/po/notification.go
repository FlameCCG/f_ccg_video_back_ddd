package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// UserGlobalNotification 用户全局通知表（纯内容，不含已读状态）.
// receiver_id > 0：个人通知；receiver_id = -1：全员广播。
// 已读状态见 NotificationUserRead（有行 = 已读）。
type UserGlobalNotification struct {
	Model
	Type             enum.Type `gorm:"index:idx_notification_receiver_type,priority:2;comment:通知类型" json:"type"`
	ReceiverID       int       `gorm:"not null;index;index:idx_notification_receiver_type,priority:1;comment:接收者用户ID,-1表示所有用户" json:"receiverID"`
	ActionUserID     uint      `gorm:"comment:操作用户ID" json:"actionUserID"`
	ActionUserAvatar string    `gorm:"size:500;comment:操作用户头像" json:"actionUserAvatar"`
	ActionUserName   string    `gorm:"size:50;comment:操作用户名" json:"actionUserName"`
	Title            string    `gorm:"size:200;comment:通知标题" json:"title"`
	Content          string    `gorm:"type:text;comment:通知内容" json:"content"`
	Link             string    `gorm:"size:500;comment:外部链接" json:"link"`
	VideoID          uint      `gorm:"comment:视频ID" json:"videoID"`
	VideoTitle       string    `gorm:"size:200;comment:视频标题" json:"videoTitle"`
	DynamicID        uint      `gorm:"comment:动态ID" json:"dynamicID"`
	CommentID        uint      `gorm:"comment:被点赞的评论ID或者回复我评论的评论ID或@我的评论ID" json:"commentID"`
}

// TableName 方法.
func (UserGlobalNotification) TableName() string {
	return "user_global_notifications"
}

// NotificationUserRead 用户通知已读记录（稀疏：有行 = 已读，无行 = 未读）.
type NotificationUserRead struct {
	UserID         uint      `gorm:"primaryKey;comment:用户ID" json:"userId"`
	NotificationID uint      `gorm:"primaryKey;index;comment:通知ID" json:"notificationId"`
	CreatedAt      time.Time `json:"createdAt"`
}

// TableName 方法.
func (NotificationUserRead) TableName() string {
	return "notification_user_reads"
}
