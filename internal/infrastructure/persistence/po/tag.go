package po

// Tag 类型.
type Tag struct {
	Model
	Name   string  `gorm:"size:64;index;comment:标签名称" json:"name"`
	Videos []Video `gorm:"many2many:video_tags" json:"videos,omitempty"` // 多对多关联视频
}

// TableName 方法.
func (Tag) TableName() string {
	return "tags"
}
