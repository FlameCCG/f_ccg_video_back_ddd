package po

// DynamicLike 动态点赞记录.
type DynamicLike struct {
	Model
	DynamicID uint `gorm:"not null;index;uniqueIndex:idx_dynamic_user,priority:1;index:idx_user_dynamic,priority:2;comment:动态ID" json:"dynamicId"`
	UserID    uint `gorm:"not null;index;uniqueIndex:idx_dynamic_user,priority:2;index:idx_user_dynamic,priority:1;comment:用户ID" json:"userId"`
}

// TableName 显式绑定表名 dynamic_likes，保持迁移与仓储 SQL 使用同一张表。
func (DynamicLike) TableName() string {
	return "dynamic_likes"
}
