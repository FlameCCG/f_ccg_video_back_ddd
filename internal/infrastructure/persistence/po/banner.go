package po

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// Banner 轮播图表.
type Banner struct {
	Model
	Cover       string          `gorm:"size:255;comment:图片链接" json:"cover"`
	Href        string          `gorm:"size:255;comment:跳转链接" json:"href"`
	Show        bool            `gorm:"default:true;comment:是否显示" json:"show"`
	Type        enum.BannerType `gorm:"default:1;comment:轮播图类型" json:"type"`
	PartitionID uint            `gorm:"default:0;comment:分区ID,仅type=1生效,0代表首页" json:"partitionId"`
}
