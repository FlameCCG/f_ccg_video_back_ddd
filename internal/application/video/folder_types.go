package video

import (
	"time"
)

// FolderWithCount 收藏夹列表项（含视频数与是否已收藏当前视频）。
type FolderWithCount struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"isDefault"`
	VideoCount  int64  `json:"videoCount"`
	IsFavorited bool   `json:"isFavorited"`
}

// FolderVideoItem 收藏夹内视频列表项。
type FolderVideoItem struct {
	ID            uint        `json:"id"`
	Title         string      `json:"title"`
	Cover         string      `json:"cover"`
	Duration      int         `json:"duration"`
	Views         int64       `json:"views"`
	DanmuCount    int64       `json:"danmuCount"`
	FavoriteCount int64       `json:"favoriteCount"`
	FavoritedAt   time.Time   `json:"favoritedAt"`
	Author        AuthorBrief `json:"author"`
}

// FolderDTO 收藏夹响应体（形态等同 po.FavoriteFolder 序列化结果）。
type FolderDTO struct {
	ID        uint   `json:"id"`
	UserID    uint   `json:"userId"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}
