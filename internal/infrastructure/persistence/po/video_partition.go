package po

// VideoPartition 视频分区表.
type VideoPartition struct {
	Model
	Name          string  `gorm:"size:50;not null;comment:分区名称" json:"name"`
	Icon          string  `gorm:"type:text;comment:分区图标SVG" json:"icon"`
	SortOrder     int     `gorm:"default:1;comment:排序权重,数字越小越靠前" json:"sortOrder"`
	IsActive      bool    `gorm:"default:true;index;comment:是否启用" json:"isActive"`
	IsSubmittable bool    `gorm:"default:true;index;comment:是否允许用户投稿" json:"isSubmittable"`
	Videos        []Video `gorm:"foreignKey:PartitionID" json:"videos,omitempty"`
}

// TableName 指定 GORM 自动迁移时使用的表名.
func (VideoPartition) TableName() string {
	return "video_partitions"
}
