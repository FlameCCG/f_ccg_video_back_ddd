// Package comment 是评论上下文的领域层：评论/评论点赞实体、评论业务规则与仓储接口。
//
// 本包为纯 Go，不依赖 gin / gorm / redis / viper / zap；
// 持久化对象（po.Comment / po.CommentLike）与本包实体的转换放在
// internal/infrastructure/persistence/comment/converter.go。
package comment

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Comment 评论实体。
//
// json tag 与原 models/mysqlEntity.Comment 的标量字段完全一致；
// 关联对象（User / Video / ParentComment）属于读侧投影，不进入领域实体。
type Comment struct {
	ID            uint             `json:"id"`
	CreatedAt     time.Time        `json:"-"`
	UpdatedAt     time.Time        `json:"-"`
	Content       string           `json:"content"`
	UserID        uint             `json:"userId"`
	VideoID       uint             `json:"videoId"`
	DynamicID     uint             `json:"dynamicId"`
	CommentType   enum.CommentType `json:"commentType"`
	ParentID      *uint            `json:"parentId"`
	RootID        *uint            `json:"rootId"`
	AtUserIDs     []uint           `json:"atUserIds"`
	Pictures      []string         `json:"pictures"`
	LikeCount     int64            `json:"likeCount"`
	ReplyCount    int64            `json:"replyCount"`
	IsPinned      bool             `json:"isPinned"`
	PinnedAt      *time.Time       `json:"pinnedAt,omitempty"`
	CommenterIP   string           `json:"commenterIP"`
	CommenterAddr string           `json:"commenterAddr"`
}

// IsRoot 报告该评论是否为一级（根）评论。
func (c *Comment) IsRoot() bool {
	return c != nil && c.RootID == nil
}

// Like 评论点赞记录实体。
//
// json tag 与原 models/mysqlEntity.CommentLike 的标量字段一致。
type Like struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	CommentID uint      `json:"commentId"`
	UserID    uint      `json:"userId"`
}

// Draft 发布评论的输入（值对象，未经规范化）。
type Draft struct {
	UserID        uint
	VideoID       uint
	DynamicID     uint
	Content       string
	ParentID      *uint
	AtUserIDs     []uint
	Pictures      []string
	CommenterIP   string
	CommenterAddr string
}

// ParentBrief 父评论投影：回复时用于推导 rootID 与被回复者信息。
type ParentBrief struct {
	UserID   uint
	RootID   *uint
	Username string
}

// NewComment 依据规范化后的草稿与推导出的根评论 ID 构造待落库的评论实体。
//
// 字段赋值顺序与原 CommentService.Create 中 mysqlEntity.Comment 字面量一致。
func NewComment(draft Draft, commentType enum.CommentType, rootID *uint) *Comment {
	return &Comment{
		Content:       draft.Content,
		UserID:        draft.UserID,
		VideoID:       draft.VideoID,
		DynamicID:     draft.DynamicID,
		CommentType:   commentType,
		ParentID:      draft.ParentID,
		RootID:        rootID,
		AtUserIDs:     draft.AtUserIDs,
		Pictures:      draft.Pictures,
		CommenterIP:   draft.CommenterIP,
		CommenterAddr: draft.CommenterAddr,
	}
}
