package video

import (
	"context"
)

// UserVideoQuery 用户投稿列表读模型查询端口。
type UserVideoQuery interface {
	// ListByAuthor 按作者用户名分页读取投稿（isOwnerView 决定可见状态集合）。
	ListByAuthor(ctx context.Context, authorUsername string, isOwnerView bool, auditStatus, sort, page, pageSize int) ([]UserVideoRow, int64, error)
	// RecentActionVideos 读取用户最近点赞/投币的视频。
	RecentActionVideos(ctx context.Context, userID uint, action string, limit int) ([]UserVideoRow, error)
	// AggregateAuthorStats 汇总作者已发布视频的行内 views/likes（含视频 ID 列表以便叠加 Redis 权威计数）。
	AggregateAuthorStats(ctx context.Context, authorUsername string) ([]AuthorVideoStatRow, error)
}
