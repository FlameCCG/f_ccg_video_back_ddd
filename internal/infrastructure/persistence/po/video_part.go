package po

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// VideoPart 视频分P（同一个作品可以包含多个分P）.
type VideoPart struct {
	Model
	VideoID     uint                  `gorm:"not null;index;comment:作品ID" json:"videoId"`
	Title       string                `gorm:"size:200;not null;comment:分P标题" json:"title"`
	SortOrder   int                   `gorm:"not null;default:0;index;comment:排序(越小越靠前)" json:"sortOrder"`
	Duration    int                   `gorm:"not null;default:0;comment:分P时长(秒)" json:"duration"`
	DanmuCount  int64                 `gorm:"default:0;comment:弹幕量" json:"danmuCount"`
	StorageType enum.VideoStorageType `gorm:"default:1;index;comment:存储类型" json:"storageType"`
	Resources   []VideoPartResource   `gorm:"foreignKey:VideoPartID" json:"resources,omitempty"`
	Video       Video                 `gorm:"foreignKey:VideoID" json:"video,omitempty"`
}

// TableName 方法.
func (VideoPart) TableName() string {
	return "video_parts"
}
