package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// DetailMeta 视频详情的文本/结构化信息（有失效时间的缓存部分）。
type DetailMeta struct {
	IsScheduled bool                        `json:"isScheduled"`
	PublishAt   *time.Time                  `json:"publishAt"`
	ApprovedAt  *time.Time                  `json:"approvedAt"`
	ID          uint                        `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Cover       string                      `json:"cover"`
	Duration    int                         `json:"duration"`
	IsOriginal  bool                        `json:"isOriginal"`
	StorageType enum.VideoStorageType       `json:"storageType"`
	Status      enum.VideoStatus            `json:"status"`
	CreatedAt   time.Time                   `json:"createdAt"`
	Partition   PartitionBrief              `json:"partition"`
	Author      AuthorBrief                 `json:"author"`
	Tags        []TagBrief                  `json:"tags"`
	Resources   []videodomain.ResourceBrief `json:"resources"`
	Parts       []VideoPartBrief            `json:"parts,omitempty"`
}

// VideoDetail 视频详情响应体。
type VideoDetail struct {
	IsScheduled   bool                        `json:"isScheduled"`
	PublishAt     *time.Time                  `json:"publishAt"`
	ApprovedAt    *time.Time                  `json:"approvedAt"`
	ID            uint                        `json:"id"`
	Title         string                      `json:"title"`
	Description   string                      `json:"description"`
	Cover         string                      `json:"cover"`
	Duration      int                         `json:"duration"`
	Views         int64                       `json:"views"`
	Likes         int64                       `json:"likes"`
	CommentCount  int64                       `json:"commentCount"`
	CoinCount     int64                       `json:"coinCount"`
	FavoriteCount int64                       `json:"favoriteCount"`
	DanmuCount    int64                       `json:"danmuCount"`
	IsOriginal    bool                        `json:"isOriginal"`
	StorageType   enum.VideoStorageType       `json:"storageType"`
	Status        enum.VideoStatus            `json:"status"`
	CreatedAt     time.Time                   `json:"createdAt"`
	Partition     PartitionBrief              `json:"partition"`
	Author        AuthorBrief                 `json:"author"`
	Tags          []TagBrief                  `json:"tags"`
	Resources     []videodomain.ResourceBrief `json:"resources"`
	Parts         []VideoPartBrief            `json:"parts,omitempty"`
	IsLiked       bool                        `json:"isLiked"`
	IsFavorited   bool                        `json:"isFavorited"`
	IsCoined      bool                        `json:"isCoined"`
}

// PartitionBrief 分区摘要。
type PartitionBrief struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// AuthorBrief 作者摘要。
type AuthorBrief struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Level       int    `json:"level"`
	Description string `json:"description"`
}

// TagBrief 标签摘要。
type TagBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// VideoPartBrief 分P摘要。
type VideoPartBrief struct {
	ID         uint                        `json:"id"`
	Title      string                      `json:"title"`
	SortOrder  int                         `json:"sortOrder"`
	Duration   int                         `json:"duration"`
	DanmuCount int64                       `json:"danmuCount"`
	Resources  []videodomain.ResourceBrief `json:"resources"`
}
