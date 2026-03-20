package entity

import (
	"slices"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Comment 评论实体。
//
// JSON 字段定义响应契约；User / Video / ParentComment 关联属于读侧投影，不进入领域实体。
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

// Like 评论点赞记录实体，JSON 字段定义接口契约。
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

// NewComment 校验并规范化草稿，复制可变输入后构造待落库实体。
func NewComment(draft Draft, rootID *uint) (*Comment, error) {
	normalized, commentType, err := draft.Normalize()
	if err != nil {
		return nil, err
	}
	draft = normalized
	return &Comment{
		Content:       draft.Content,
		UserID:        draft.UserID,
		VideoID:       draft.VideoID,
		DynamicID:     draft.DynamicID,
		CommentType:   commentType,
		ParentID:      copyCommentID(draft.ParentID),
		RootID:        copyCommentID(rootID),
		AtUserIDs:     slices.Clone(draft.AtUserIDs),
		Pictures:      slices.Clone(draft.Pictures),
		CommenterIP:   draft.CommenterIP,
		CommenterAddr: draft.CommenterAddr,
	}, nil
}

func copyCommentID(id *uint) *uint {
	if id == nil {
		return nil
	}
	value := *id
	return &value
}

// SetPinned 同时维护置顶标记和时间；作品作者的身份事实由用例取得。
// 同一作品只能有一条置顶评论，由调用方在同一事务内解除旧置顶并保存本实体。
func (c *Comment) SetPinned(pinned bool, now time.Time) error {
	if err := c.EnsurePinnable(); err != nil {
		return err
	}
	c.IsPinned = pinned
	c.PinnedAt = nil
	if pinned {
		c.PinnedAt = &now
	}
	return nil
}
