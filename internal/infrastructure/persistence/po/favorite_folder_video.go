package po

import "time"

// FavoriteFolderVideo 收藏夹-视频 关联表.
type FavoriteFolderVideo struct {
	FavoriteFolderID uint      `gorm:"primaryKey" json:"favoriteFolderId"`
	VideoID          uint      `gorm:"primaryKey" json:"videoId"`
	CreatedAt        time.Time `gorm:"index" json:"-"`
}

// TableName 显式绑定表名 favorite_folder_videos，保持迁移与仓储 SQL 使用同一张表。
func (FavoriteFolderVideo) TableName() string {
	return "favorite_folder_videos"
}
