package po

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// VideoResource 视频资源表(支持多分辨率).
type VideoResource struct {
	Model
	VideoID        uint                 `gorm:"not null;index;uniqueIndex:uk_video_resources_variant,priority:1;comment:视频ID" json:"videoId"`
	Resolution     enum.VideoResolution `gorm:"size:64;not null;uniqueIndex:uk_video_resources_variant,priority:2;comment:分辨率" json:"resolution"`
	SourceFileName string               `gorm:"size:255;not null;comment:源文件名" json:"sourceFileName"`
	FileURL        string               `gorm:"size:500;not null;comment:文件URL" json:"fileUrl"`
	FileSize       int64                `gorm:"not null;comment:文件大小(字节)" json:"fileSize"`
	Bitrate        int                  `gorm:"not null;comment:码率(kbps)" json:"bitrate"`
	Format         string               `gorm:"size:10;not null;comment:文件格式(mp4, m3u8等)" json:"format"`
	Codec          string               `gorm:"size:20;comment:编码格式" json:"codec"`
	IsVip          bool                 `gorm:"not null;default:false;comment:是否是Vip资源" json:"isVip"`
	IsSouceVideo   bool                 `gorm:"not null;default:false;uniqueIndex:uk_video_resources_variant,priority:3;comment:是否是源视频" json:"isSouceVideo"`
	// 关联关系
	Video Video `gorm:"foreignKey:VideoID" json:"video,omitempty"`
}

// TableName 方法.
func (VideoResource) TableName() string {
	return "video_resources"
}
