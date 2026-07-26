package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// Comment 评论表.
type Comment struct {
	Model
	Content       string           `gorm:"type:text;not null;comment:评论内容" json:"content"`
	UserID        uint             `gorm:"not null;index;comment:评论用户ID" json:"userId"`
	VideoID       uint             `gorm:"not null;default:0;index;comment:视频ID" json:"videoId"`
	DynamicID     uint             `gorm:"not null;default:0;index;comment:动态ID" json:"dynamicId"`
	CommentType   enum.CommentType `gorm:"not null;comment:评论类型" json:"commentType"`
	ParentID      *uint            `gorm:"index;comment:父评论ID(用于回复)" json:"parentId"`
	RootID        *uint            `gorm:"index;comment:根评论ID" json:"rootId"`
	AtUserIDs     []uint           `gorm:"type:json;serializer:json;comment:@的用户ID列表" json:"atUserIds"`
	Pictures      []string         `gorm:"type:json;serializer:json;comment:评论附带图片列表" json:"pictures"`
	LikeCount     int64            `gorm:"default:0;comment:点赞数" json:"likeCount"`
	ReplyCount    int64            `gorm:"default:0;comment:回复数" json:"replyCount"`
	IsPinned      bool             `gorm:"default:false;index;comment:是否置顶" json:"isPinned"`
	PinnedAt      *time.Time       `gorm:"comment:置顶时间" json:"pinnedAt,omitempty"`
	CommenterIP   string           `gorm:"type:varchar(20);comment:评论者IP" json:"commenterIP"`
	CommenterAddr string           `gorm:"type:varchar(255);comment:评论者地址" json:"commenterAddr"`
	// 关联关系
	User          User          `gorm:"foreignKey:UserID" json:"userEntity,omitempty"`
	Video         Video         `gorm:"foreignKey:VideoID" json:"video,omitempty"`
	Dynamic       Dynamic       `gorm:"foreignKey:DynamicID" json:"dynamic,omitempty"`
	ParentComment *Comment      `gorm:"foreignKey:ParentID" json:"parentComment,omitempty"`
	RootComment   *Comment      `gorm:"foreignKey:RootID" json:"rootComment,omitempty"`
	Children      []Comment     `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	CommentLike   []CommentLike `gorm:"foreignKey:CommentID" json:"commentLikes,omitempty"`
}

// TableName 方法.
func (Comment) TableName() string {
	return "comments"
}
