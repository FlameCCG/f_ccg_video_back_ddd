// Package comment 表达评论实体、回复约束、点赞意图和仓储契约。
//
// comment.go/rules.go 回答“是否允许创建、回复或操作”，repository.go 表达保存实体所需能力。
// PO 到领域对象的转换在 persistence/comment；页面列表通过应用查询模型构建，不直接复用落库对象。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
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
