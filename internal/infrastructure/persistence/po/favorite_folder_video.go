package po

import "time"

// FavoriteFolderVideo 收藏夹-视频 关联表.
type FavoriteFolderVideo struct {
	FavoriteFolderID uint      `gorm:"primaryKey" json:"favoriteFolderId"`
	VideoID          uint      `gorm:"primaryKey" json:"videoId"`
	CreatedAt        time.Time `gorm:"index" json:"-"`
}

// TableName 方法.
func (FavoriteFolderVideo) TableName() string {
	return "favorite_folder_videos"
}
