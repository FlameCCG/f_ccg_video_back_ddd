package identity

import (
	"context"
)

// CreatorMetricStore 创作中心指标存储端口，通过 Redis 哈希按天累加。
//
// Redis key 格式属于外部契约，由 infrastructure 适配器实现。
type CreatorMetricStore interface {
	// Available 报告底层存储是否可用；不可用时调用方返回对应的存储错误。
	Available() error
	// Incr 累加指定指标在当天的计数。
	Incr(ctx context.Context, metric string, creatorID uint, delta int64) error
	// Series 按天字段批量读取指标值，缺失位置补 0。
	Series(ctx context.Context, metric string, creatorID uint, dayFields []string) ([]int64, error)
}

// CreatorVideoStatsQuery 作者已发布视频统计汇总端口（读侧投影）。
type CreatorVideoStatsQuery interface {
	AggregateByAuthor(ctx context.Context, username string) (CreatorVideoAggregate, error)
}

// FansCounter 粉丝数查询端口。
//
// 由 social 上下文的应用服务在 bootstrap 中绑定实现，实现跨上下文协调。
type FansCounter interface {
	FansCount(ctx context.Context, userID uint) int64
}
