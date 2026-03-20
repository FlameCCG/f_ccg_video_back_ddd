package video

import (
	"context"
)

// FolderQuery 收藏夹读模型查询端口。
type FolderQuery interface {
	// ListWithCount 读取用户收藏夹列表（含视频数与「该视频是否已在此夹」）。
	ListWithCount(ctx context.Context, userID, videoID uint) ([]FolderWithCount, error)
	// ListVideos 读取收藏夹内的视频列表。
	ListVideos(ctx context.Context, folderID uint, page, pageSize, sort int) ([]FolderVideoItem, int64, error)
}

// FolderListCache 收藏夹列表的 Redis 缓存端口。
type FolderListCache interface {
	Load(userID, videoID uint) ([]FolderWithCount, bool)
	Store(userID, videoID uint, items []FolderWithCount)
	Invalidate(userID uint)
}
