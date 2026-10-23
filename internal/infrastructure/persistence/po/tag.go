package po

// Tag 是可关联多个视频的标签行，Videos 经 video_tags 中间表建立多对多关系。
type Tag struct {
	Model
	Name   string  `gorm:"size:64;index;comment:标签名称" json:"name"`
	Videos []Video `gorm:"many2many:video_tags" json:"videos,omitempty"` // 多对多关联视频
}

// TableName 显式绑定表名 tags，保持迁移与仓储 SQL 使用同一张表。
func (Tag) TableName() string {
	return "tags"
}
