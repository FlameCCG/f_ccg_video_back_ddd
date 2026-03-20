package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// UpdateVideoOpts 改稿参数。
type UpdateVideoOpts struct {
	VideoID     uint
	Title       *string
	Description *string
	PartitionID *uint
	TagIDs      []uint
	IsOriginal  *bool
	IsPrivate   *bool
	CoverURL    *string
	PublishTime *time.Time
}

// UpdatedVideo 改稿后的视频快照。
//
// 响应始终包含 author 与 category 对象，包括其零值形态。
type UpdatedVideo struct {
	ID             uint                  `json:"id"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	Cover          string                `json:"cover"`
	Duration       int                   `json:"duration"`
	AuthorUsername string                `json:"authorUsername"`
	PartitionID    uint                  `json:"partitionID"`
	Views          int64                 `json:"views"`
	Likes          int64                 `json:"likes"`
	CommentCount   int64                 `json:"commentCount"`
	CoinCount      int64                 `json:"coinCount"`
	FavoriteCount  int64                 `json:"favoriteCount"`
	DanmuCount     int64                 `json:"danmuCount"`
	IsPinned       bool                  `json:"isPinned"`
	PinnedAt       *time.Time            `json:"pinnedAt,omitempty"`
	Tags           []UpdatedVideoTag     `json:"tags"`
	IsOriginal     bool                  `json:"isOriginal"`
	StorageType    enum.VideoStorageType `json:"storageType"`
	Status         enum.VideoStatus      `json:"status"`
	Author         UpdatedVideoAuthor    `json:"author,omitempty"`
	Partition      UpdatedVideoPartition `json:"category,omitempty"`
}

// UpdatedVideoTag 改稿响应中的标签（形态等同 po.Tag 序列化结果）。
type UpdatedVideoTag struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// UpdatedVideoAuthor 改稿响应中的作者对象。
//
// Author 不做预加载，字段为零值，按用户对象的 JSON 结构序列化。
type UpdatedVideoAuthor struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       *time.Time      `json:"birthday"`
	Level          int             `json:"level"`
	OpenID         string          `json:"openID"`
	RegisterSource string          `json:"registerSource"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	Status         enum.UserStatus `json:"status"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterIP     string          `json:"registerIP"`
	LastLoginIP    string          `json:"lastLoginIP"`
	RegisterAddr   string          `json:"registerAddr"`
	LastLoginAddr  string          `json:"lastLoginAddr"`
}

// UpdatedVideoPartition 改稿响应中的分区对象（形态等同 po.VideoPartition 序列化结果）。
type UpdatedVideoPartition struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sortOrder"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}
