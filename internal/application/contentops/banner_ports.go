package contentops

import (
	"context"

	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BannerCache 可见轮播图缓存端口（Redis 实现在 infrastructure/cache/contentops）。
//
// 端口使用固定 300ms 独立超时，避免请求取消中断缓存回填。Redis key 前缀与 TTL 属于外部契约。
type BannerCache interface {
	// LoadVisible 读取可见轮播图缓存；未命中或反序列化失败返回 (nil, false)。
	LoadVisible(bannerType enum.BannerType, partitionID *uint) ([]contentopsdomain.Banner, bool)
	// StoreVisible 回填可见轮播图缓存（失败仅记录日志）。
	StoreVisible(bannerType enum.BannerType, partitionID *uint, items []contentopsdomain.Banner)
	// InvalidateVisible 按前缀清空全部可见轮播图缓存。
	InvalidateVisible()
}

// VideoPartitionQuery 视频分区存在性查询端口。
//
// 跨上下文读侧投影：分区聚合归 video 上下文所有，这里只做一次 COUNT 校验，
// 允许在本上下文的 infrastructure 中直读 video_partitions 表。
type VideoPartitionQuery interface {
	// Exists 报告分区是否存在。
	Exists(ctx context.Context, partitionID uint) (bool, error)
}
