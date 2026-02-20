package po

// FavoriteFolder 收藏夹.
type FavoriteFolder struct {
	Model
	UserID    uint     `gorm:"not null;index;comment:用户ID" json:"userId"`
	Name      string   `gorm:"size:50;not null;comment:收藏夹名称" json:"name"`
	IsDefault bool     `gorm:"not null;default:false;comment:是否默认收藏夹" json:"isDefault"`
	Videos    []*Video `gorm:"many2many:favorite_folder_videos" json:"videos,omitempty"`
}

// TableName 方法.
func (FavoriteFolder) TableName() string {
	return "favorite_folders"
}
