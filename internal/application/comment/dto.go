// Package comment 编排评论创建、回复、点赞与管理。
//
// comment_service.go 组合评论仓储、目标查询、点赞缓存与通知端口，admin_service.go 处理管理端操作。
// 评论持久化、Redis 点赞和通知各有失败边界；不能从一次 HTTP 成功推断所有派生计数都已回写。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package comment

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// CommentItem 评论返回结构（读模型，json tag 与原 comment_service.CommentItem 逐字一致）.
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

// ReplyInfo 被回复者信息（json tag 与原 comment_service.ReplyInfo 一致）.
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

// CreateCommand 发布评论命令（由 interfaces 层组装，IP 与归属地在 handler 提取）。
type CreateCommand struct {
	UserID    uint
	VideoID   uint
	DynamicID uint
	Content   string
	ParentID  *uint
	AtUserIDs []uint
	Pictures  []string
	IP        string
	Addr      string
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

// UserProfile 用户投影（id / username / avatar）。
type UserProfile struct {
	ID       uint
	Username string
	Avatar   string
}

// VideoBrief 视频投影。
//
// 不同查询方法填充的字段不同：PublishedVideoBrief 填 ID/Title/AuthorUsername，
// VideoBrief 只填 ID/Title（与原实现各自的 Select 列表一致）。
type VideoBrief struct {
	ID             uint
	Title          string
	AuthorUsername string
}

// VideoAuthorRef 视频作者引用投影（用于删除/置顶鉴权）。
type VideoAuthorRef struct {
	VideoID uint
	// AuthorUserID 为作者用户 ID；作者用户记录缺失时为 0。
	AuthorUserID uint
}

// DynamicBrief 动态投影（id / 发布者 ID）。
type DynamicBrief struct {
	ID     uint
	UserID uint
}

// Notice 通知写入载荷（跨上下文端口 Notifier 的入参）。
//
// 字段与原 models/mysqlEntity.UserGlobalNotification 中评论模块实际写入的列一一对应，
// 保证落库内容与原实现逐字段一致。
type Notice struct {
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
	CommentID        uint
}
