package po

// Menu 菜单表（极简版 - 只管前端显示）.
type Menu struct {
	Model
	Title     string  `gorm:"size:100;not null;comment:菜单标题" json:"title"`
	TitleEn   string  `gorm:"size:100;comment:菜单英文标题" json:"titleEn"`
	TitleJa   string  `gorm:"size:100;comment:菜单日文标题" json:"titleJa"`
	Icon      string  `gorm:"size:2048;comment:菜单图标SVG" json:"icon"`
	Path      string  `gorm:"size:255;not null;default:'';comment:路由路径" json:"path"`
	Name      string  `gorm:"size:100;not null;default:'';comment:路由名称" json:"name"`
	Component string  `gorm:"size:255;not null;default:'';comment:前端组件路径/标识" json:"component"`
	KeepAlive bool    `gorm:"not null;default:0;comment:是否缓存" json:"keepAlive"`
	ParentID  *uint   `gorm:"index;comment:父菜单ID" json:"parentId"`
	SortOrder uint    `gorm:"comment:排序" json:"sortOrder"`
	Roles     []*Role `gorm:"many2many:role_menus;" json:"roles,omitempty"` // 多对多关系
	Children  []*Menu `gorm:"-" json:"children,omitempty"`                  // 子菜单，不存数据库
}

// TableName 显式绑定表名 menus，保持迁移与仓储 SQL 使用同一张表。
func (Menu) TableName() string {
	return "menus"
}
