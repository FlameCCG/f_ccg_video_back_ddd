package po

// DanmuLike 弹幕点赞记录（用户-弹幕）.
type DanmuLike struct {
	Model
	DanmuID uint `gorm:"not null;index;uniqueIndex:idx_danmu_user,priority:1;comment:弹幕ID" json:"danmuId"`
	UserID  uint `gorm:"not null;index;uniqueIndex:idx_danmu_user,priority:2;comment:用户ID" json:"userId"`
}

// TableName 显式绑定表名 danmu_likes，保持迁移与仓储 SQL 使用同一张表。
func (DanmuLike) TableName() string {
	return "danmu_likes"
}
