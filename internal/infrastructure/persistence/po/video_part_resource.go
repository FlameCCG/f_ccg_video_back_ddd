package po

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// VideoPartResource 分P资源表(支持多分辨率).
//
// 唯一索引组合 分P ID、Resolution 和 IsSouceVideo，支持重试时按资源变体复用/更新。
// IsSouceVideo 的历史拼写同时进入列名与 JSON，不应在注释整理中顺带更名。
// 自适应资源的 FileURL 指向 MPD 清单，一行数据库记录可能对应存储中的一整个目录。
type VideoPartResource struct {
	Model
	VideoPartID    uint                 `gorm:"not null;index;uniqueIndex:uk_video_part_resources_variant,priority:1;comment:分P ID" json:"videoPartId"`
	Resolution     enum.VideoResolution `gorm:"size:64;not null;uniqueIndex:uk_video_part_resources_variant,priority:2;comment:分辨率" json:"resolution"`
	SourceFileName string               `gorm:"size:255;not null;comment:源文件名" json:"sourceFileName"`
	FileURL        string               `gorm:"size:500;not null;comment:文件URL" json:"fileUrl"`
	FileSize       int64                `gorm:"not null;comment:文件大小(字节)" json:"fileSize"`
	Bitrate        int                  `gorm:"not null;comment:码率(kbps)" json:"bitrate"`
	Format         string               `gorm:"size:10;not null;comment:文件格式(mp4, m3u8等)" json:"format"`
	Codec          string               `gorm:"size:20;comment:编码格式" json:"codec"`
	IsVip          bool                 `gorm:"not null;default:false;comment:是否是Vip资源" json:"isVip"`
	IsSouceVideo   bool                 `gorm:"not null;default:false;uniqueIndex:uk_video_part_resources_variant,priority:3;comment:是否是源视频" json:"isSouceVideo"`

	VideoPart Part `gorm:"foreignKey:VideoPartID" json:"videoPart,omitempty"`
}

// TableName 显式绑定表名 video_part_resources，保持迁移与仓储 SQL 使用同一张表。
func (VideoPartResource) TableName() string {
	return "video_part_resources"
}
