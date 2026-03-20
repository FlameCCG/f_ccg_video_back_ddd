package dynamic

import (
	"time"

	dynamicvo "f_ccg_video_back_ddd/internal/domain/dynamic/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

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

// FeedRow 作品流读侧投影，动态与视频使用同一批查询列。
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
	Sources        dynamicvo.FeedSources
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
	Sources        dynamicvo.FeedSources
	IsOwnerView    bool
}

// FollowingFeedParams 当前用户关注聚合流查询参数。
type FollowingFeedParams struct {
	ViewerID uint
	Sources  dynamicvo.FeedSources
	Offset   int
	Limit    int
}

// FollowingFeedCountParams 当前用户关注聚合流计数参数。
type FollowingFeedCountParams struct {
	ViewerID uint
	Sources  dynamicvo.FeedSources
}
