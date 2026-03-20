package video

import (
	"context"
)

// VideoVectorIndexer 视频向量索引端口（最佳努力，失败仅记录日志）。
//
// 绑定目标：search 上下文。
type VideoVectorIndexer interface {
	IndexVideoVectorBestEffort(ctx context.Context, videoID uint)
	DeleteVideoVectorBestEffort(ctx context.Context, videoID uint)
}

// VectorIndexRebuilder 向量索引全量重建端口。
//
// 绑定目标：search 上下文。
// 已有任务在跑时约定返回 (false, nil)，由应用层转成
// ErrVectorRebuildInProgress 的错误文案，避免跨上下文共享哨兵错误。
type VectorIndexRebuilder interface {
	StartRebuild(opts VectorRebuildOptions) (started bool, err error)
	RebuildStatus() any
}

// VectorRebuildOptions 向量索引重建参数。
type VectorRebuildOptions struct {
	// Recreate 为 true 时先删除索引再按当前配置重建。
	Recreate bool
	// Workers 并发向量化数；<=0 时由实现取默认值。
	Workers int
}
