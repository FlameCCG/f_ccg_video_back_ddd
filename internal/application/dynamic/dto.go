// Package dynamic 是动态上下文的应用层：用例服务、DTO（含读模型）与查询/协作端口。
//
// 本包不依赖 gin / gorm / redis：数据访问一律经端口，实现在 infrastructure。
package dynamic

import (
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// DynamicItem 动态列表项（读模型，json tag 与原 dynamic_service.DynamicItem 逐字一致）.
type DynamicItem struct {
	ID        uint       `json:"id"`
	UserID    uint       `json:"userId"`
	Username  string     `json:"username"`
	Avatar    string     `json:"avatar"`
	Content   string     `json:"content"`
	ImageURL  string     `json:"imageUrl"`
	IsPinned  bool       `json:"isPinned"`
	PinnedAt  *time.Time `json:"pinnedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// AuthorBrief 作品流中的作者摘要（读模型）.
type AuthorBrief struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// VideoBrief 作品流中的视频摘要（读模型）.
type VideoBrief struct {
	ID           uint       `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Cover        string     `json:"cover"`
	Duration     int        `json:"duration"`
	Views        int64      `json:"views"`
	DanmuCount   int64      `json:"danmuCount"`
	LikeCount    int64      `json:"likeCount"`
	CommentCount int64      `json:"commentCount"`
	IsLiked      bool       `json:"isLiked"`
	IsPinned     bool       `json:"isPinned"`
	PinnedAt     *time.Time `json:"pinnedAt,omitempty"`
}

// DynamicBrief 作品流中的动态摘要（读模型）.
type DynamicBrief struct {
	ID           uint       `json:"id"`
	Content      string     `json:"content"`
	ImageURL     string     `json:"imageUrl"`
	LikeCount    int64      `json:"likeCount"`
	CommentCount int64      `json:"commentCount"`
	IsLiked      bool       `json:"isLiked"`
	IsPinned     bool       `json:"isPinned"`
	PinnedAt     *time.Time `json:"pinnedAt,omitempty"`
}

// WorkFeedItem 作品流条目（视频与动态二选一，读模型）.
type WorkFeedItem struct {
	WorkType  enum.WorkType `json:"workType"`
	CreatedAt time.Time     `json:"createdAt"`
	Author    AuthorBrief   `json:"author"`
	Video     *VideoBrief   `json:"video,omitempty"`
	Dynamic   *DynamicBrief `json:"dynamic,omitempty"`
}

// FollowUserItem 关注列表条目（读模型）.
type FollowUserItem struct {
	UserID         uint          `json:"userId"`
	Username       string        `json:"username"`
	Avatar         string        `json:"avatar"`
	FollowedAt     time.Time     `json:"followedAt"`
	LatestWorkAt   *time.Time    `json:"latestWorkAt,omitempty"`
	LatestWorkType enum.WorkType `json:"latestWorkType,omitempty"`
	IsUnread       bool          `json:"isUnread"`
}

// DynamicLikeResult 动态点赞切换结果（读模型）.
type DynamicLikeResult struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// AdminDynamicListParams 管理端动态列表查询参数.
type AdminDynamicListParams struct {
	UserID   uint
	Username string
	Keyword  string
	Page     int
	PageSize int
}

// FeedRow 作品流联合查询的原始行（application 内部读模型，不直接对外序列化）。
//
// 字段与原 dynamic_service.feedRow 一一对应；动态与视频共用同一批列，
// 其中 DynIsPinned / DynPinnedAt 两列对两类作品都表示「置顶状态」。
type FeedRow struct {
	WorkType       int8
	WorkID         uint
	CreatedAt      time.Time
	AuthorID       uint
	AuthorUsername string
	AuthorAvatar   string
	VideoTitle     string
	VideoDesc      string
	VideoCover     string
	VideoDuration  int
	VideoViews     int64
	VideoDanmu     int64
	VideoLikeCount int64
	VideoComment   int64
	DynContent     string
	DynImageURL    string
	DynLikeCount   int64
	DynIsPinned    bool
	DynPinnedAt    *time.Time
}

// AuthorFeedParams 作者作品流查询参数。
type AuthorFeedParams struct {
	AuthorID       uint
	AuthorUsername string
	AuthorAvatar   string
	Sources        domain.FeedSources
	// IsOwnerView 作者本人查看自己主页时可见未发布视频。
	IsOwnerView bool
	Offset      int
	// Limit 实际取行上限，调用方会多取一行用于探测是否存在下一页。
	Limit int
}

// AuthorFeedCountParams 作者作品流计数参数。
type AuthorFeedCountParams struct {
	AuthorID       uint
	AuthorUsername string
	Sources        domain.FeedSources
	IsOwnerView    bool
}

// FollowRelation 当前用户的一条关注关系（含被关注者资料）。
type FollowRelation struct {
	FocusUserID uint
	FollowedAt  time.Time
	Username    string
	Avatar      string
}

// UserProfile 用户资料摘要（id / username / avatar）。
type UserProfile struct {
	ID       uint
	Username string
	Avatar   string
}

// VideoAuthorRef 已发布视频的作者引用（原 MaybeMarkReadByVideo 的联表结果）。
type VideoAuthorRef struct {
	VideoID        uint
	VideoAt        time.Time
	AuthorID       uint
	AuthorUsername string
	VideoStatus    int
}
