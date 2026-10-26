package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// Video 视频主表.
//
// Status 决定稿件状态；PublishAt 是计划时间，ApprovedAt 是审核事实，IsScheduled 保留定时作品属性。
// 它们不能合并成一个状态字段：定时稿可以待审，已发布定时稿也需要保留原计划供后台查看。
// 这些 GORM 关联描述表关系，并不意味着每个查询都预加载全部对象。
type Video struct {
	Model
	Title          string                `gorm:"size:200;not null;index;comment:视频标题" json:"title"`
	Description    string                `gorm:"type:text;comment:视频描述" json:"description"`
	Cover          string                `gorm:"size:500;comment:封面图片URL" json:"cover"`
	Duration       int                   `gorm:"not null;default:0;comment:视频时长(秒)" json:"duration"`
	AuthorUsername string                `gorm:"size:50;not null;index;index:idx_videos_author_status,priority:1;comment:上传用户名" json:"authorUsername"`
	PartitionID    uint                  `gorm:"not null;index;comment:所属分区ID" json:"partitionID"`
	Views          int64                 `gorm:"default:0;index;comment:播放量" json:"views"`
	Likes          int64                 `gorm:"default:0;comment:点赞量" json:"likes"`
	CommentCount   int64                 `gorm:"default:0;comment:评论量" json:"commentCount"`
	CoinCount      int64                 `gorm:"default:0;comment:投币量" json:"coinCount"`
	FavoriteCount  int64                 `gorm:"default:0;comment:收藏量" json:"favoriteCount"`
	DanmuCount     int64                 `gorm:"default:0;comment:弹幕量" json:"danmuCount"`
	IsPinned       bool                  `gorm:"default:false;index;comment:是否置顶(动态模块)" json:"isPinned"`
	PinnedAt       *time.Time            `gorm:"comment:置顶时间(动态模块)" json:"pinnedAt,omitempty"`
	Tags           []Tag                 `gorm:"many2many:video_tags" json:"tags"`
	IsOriginal     bool                  `gorm:"default:true;comment:是否原创" json:"isOriginal"`
	StorageType    enum.VideoStorageType `gorm:"default:1;index;comment:存储类型" json:"storageType"`
	Status         enum.VideoStatus      `gorm:"default:1;index;index:idx_videos_author_status,priority:2;comment:视频状态" json:"status"`
	PublishAt      *time.Time            `gorm:"index:idx_videos_publish_at;comment:定时发布时间" json:"publishAt,omitempty"`
	ApprovedAt     *time.Time            `gorm:"comment:审核通过时间" json:"approvedAt,omitempty"`
	IsScheduled    bool                  `gorm:"not null;default:false;comment:是否定时作品" json:"isScheduled"`
	Author         User                  `gorm:"foreignKey:AuthorUsername;references:Username" json:"author,omitempty"`
	VideoPartition VideoPartition        `gorm:"foreignKey:PartitionID" json:"category,omitempty"`
	VideoResources []VideoResource       `gorm:"foreignKey:VideoID" json:"videoResources,omitempty"`
	VideoParts     []Part                `gorm:"foreignKey:VideoID" json:"videoParts,omitempty"`
	VideoComments  []Comment             `gorm:"foreignKey:VideoID" json:"videoComments,omitempty"`
	Danmu          []Danmu               `gorm:"foreignKey:VideoID" json:"danmu,omitempty"`
	FavoriteFolder []FavoriteFolder      `gorm:"many2many:favorite_folder_videos" json:"favoriteFolders,omitempty"`
}

// TableName 显式绑定表名 videos，保持迁移与仓储 SQL 使用同一张表。
func (Video) TableName() string {
	return "videos"
}
