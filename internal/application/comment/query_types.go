package comment

import (
	"time"
)

// CommentItem 评论返回结构（读模型）.
type CommentItem struct {
	ID            uint       `json:"id"`
	Content       string     `json:"content"`
	UserID        uint       `json:"userId"`
	Username      string     `json:"username"`
	Avatar        string     `json:"avatar"`
	VideoID       uint       `json:"videoId"`
	DynamicID     uint       `json:"dynamicId"`
	VideoTitle    string     `json:"videoTitle,omitempty"`
	VideoCover    string     `json:"videoCover,omitempty"`
	ParentID      *uint      `json:"parentId"`
	RootID        *uint      `json:"rootId"`
	AtUserIDs     []uint     `json:"atUserIds"`
	Pictures      []string   `json:"pictures,omitempty"`
	LikeCount     int64      `json:"likeCount"`
	ReplyCount    int64      `json:"replyCount,omitempty"`
	IsPinned      bool       `json:"isPinned"`
	PinnedAt      *time.Time `json:"pinnedAt,omitempty"`
	CommenterAddr string     `json:"commenterAddr"`
	CreatedAt     time.Time  `json:"createdAt"`
	IsLiked       bool       `json:"isLiked"`
	ReplyTo       *ReplyInfo `json:"replyTo,omitempty"`
}

// ReplyInfo 被回复者信息.
type ReplyInfo struct {
	UserID   uint   `json:"userId"`
	Username string `json:"username"`
}

// CommentRow 评论列表行（读模型：评论标量字段 + 关联用户/视频/父评论投影）。
//
// 由查询端口的 infrastructure 实现直接组装，应用层再叠加点赞/回复数缓存与点赞状态。
type CommentRow struct {
	ID            uint
	Content       string
	UserID        uint
	Username      string
	Avatar        string
	VideoID       uint
	DynamicID     uint
	VideoTitle    string
	VideoCover    string
	ParentID      *uint
	RootID        *uint
	AtUserIDs     []uint
	Pictures      []string
	LikeCount     int64
	ReplyCount    int64
	IsPinned      bool
	PinnedAt      *time.Time
	CommenterAddr string
	CreatedAt     time.Time
	ReplyTo       *ReplyInfo
}

// ListParams 根评论列表用例参数（ViewerID 为 0 表示未登录）。
type ListParams struct {
	VideoID   uint
	DynamicID uint
	ViewerID  uint
	Page      int
	PageSize  int
	SortBy    string
}

// RepliesParams 回复列表用例参数。
type RepliesParams struct {
	RootID   uint
	ViewerID uint
	Page     int
	PageSize int
}

// CreatorListParams 创作中心评论列表用例参数（Sort：0 最近 / 1 点赞最多 / 2 回复最多）。
type CreatorListParams struct {
	AuthorID uint
	Page     int
	PageSize int
	Sort     int
	Keyword  string
}

// AdminListParams 管理端评论列表用例参数（Sort：latest / oldest / likes / replies）。
type AdminListParams struct {
	UserID   uint
	Username string
	Keyword  string
	Sort     string
	Page     int
	PageSize int
}

// RootListQuery 根评论列表查询参数（分页已纠偏）。
type RootListQuery struct {
	VideoID   uint
	DynamicID uint
	Page      int
	PageSize  int
	SortBy    string
}

// ReplyListQuery 回复列表查询参数（分页已纠偏）。
type ReplyListQuery struct {
	RootID   uint
	Page     int
	PageSize int
}

// CreatorListQuery 创作中心评论列表查询参数（分页已纠偏，作者名已解析）。
type CreatorListQuery struct {
	AuthorUsername string
	Page           int
	PageSize       int
	Sort           int
	Keyword        string
}

// AdminListQuery 管理端评论列表查询参数（分页已纠偏，关键字已去空白）。
type AdminListQuery struct {
	UserID   uint
	Username string
	Keyword  string
	Sort     string
	Page     int
	PageSize int
}
