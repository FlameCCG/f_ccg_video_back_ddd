package video

import (
	"context"
)

// VideoFactsQuery 当前用例需要的视频及作者辅助事实，允许跨表投影。
type VideoFactsQuery interface {
	// EnsurePublished 校验视频存在且处于已发布状态；否则返回 ErrRecordNotFound。
	EnsurePublished(ctx context.Context, videoID uint) error
	// Exists 判断视频行是否存在（不限状态）。
	Exists(ctx context.Context, videoID uint) (bool, error)
	// AuthorIDByVideoID 读取视频作者的用户 ID；作者缺失时返回 ErrVideoAuthorNotFound。
	AuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error)
	// PublishedAuthorIDByVideoID 读取已发布视频作者的用户 ID（供事务内互动流程使用）。
	PublishedAuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error)
	// AuthorUsernameByVideoID 读取视频的作者用户名；视频不存在时返回 ErrRecordNotFound。
	AuthorUsernameByVideoID(ctx context.Context, videoID uint) (string, error)
	// UsernameByUserID 读取用户名（读侧投影，可直读 users 表）。
	UsernameByUserID(ctx context.Context, userID uint) (string, error)
	// UserIDByUsername 读取用户 ID 与规范化后的用户名（读侧投影）。
	UserIDByUsername(ctx context.Context, username string) (uint, string, error)
	// IDsByAuthor 读取该作者名下的全部视频 ID（用于批量失效详情缓存）。
	IDsByAuthor(ctx context.Context, authorUsername string) ([]uint, error)
	// IDsByPartition 列出某分区下的全部视频 ID。
	IDsByPartition(ctx context.Context, partitionID uint) ([]uint, error)
	// CountByPartition 统计某分区下的视频数量。
	CountByPartition(ctx context.Context, partitionID uint) (int64, error)
}
