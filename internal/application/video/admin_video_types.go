package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AdminVideoAuthor 管理端视频列表的作者信息。
type AdminVideoAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// AdminVideoItem 管理端视频列表项。
type AdminVideoItem struct {
	IsScheduled   bool             `json:"isScheduled"`
	PublishAt     *time.Time       `json:"publishAt"`
	ApprovedAt    *time.Time       `json:"approvedAt"`
	ID            uint             `json:"id"`
	Title         string           `json:"title"`
	Cover         string           `json:"cover"`
	Duration      int              `json:"duration"`
	Views         int64            `json:"views"`
	Likes         int64            `json:"likes"`
	CommentCount  int64            `json:"commentCount"`
	CoinCount     int64            `json:"coinCount"`
	FavoriteCount int64            `json:"favoriteCount"`
	DanmuCount    int64            `json:"danmuCount"`
	IsOriginal    bool             `json:"isOriginal"`
	StorageType   string           `json:"storageType"`
	Status        enum.VideoStatus `json:"status"`
	PartitionID   uint             `json:"partitionId"`
	PartitionName string           `json:"partitionName"`
	Author        AdminVideoAuthor `json:"author"`
	CreatedAt     time.Time        `json:"createdAt"`
}

// AdminVideoListParams 管理端视频列表查询参数。
type AdminVideoListParams struct {
	IsScheduled *bool
	UserID      uint
	Keyword     string
	Status      int
	PartitionID uint
	Sort        string // latest/oldest/views/likes/duration
	Page        int
	PageSize    int
	// AuthorUsername 由应用层按 UserID 解析后填充（读侧投影）。
	AuthorUsername string
}

// AdminVideoReviewParams 管理端审核参数。
type AdminVideoReviewParams struct {
	VideoIDs []uint
	Status   enum.VideoStatus
}
