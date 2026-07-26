package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AdminUserQuery 管理端用户列表查询端口（读侧，实现在 infrastructure）。
type AdminUserQuery interface {
	List(ctx context.Context, params AdminUserListParams) ([]AdminUserListItem, int64, error)
}

// BanRecordQuery 管理端封禁记录查询端口（读侧）。
type BanRecordQuery interface {
	AdminList(ctx context.Context, params AdminBanRecordListParams) ([]AdminBanRecordItem, int64, error)
}

// CoinRecordQuery 硬币流水查询端口（读侧）。
type CoinRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]CoinRecordItem, int64, error)
}

// ExpRecordQuery 经验流水查询端口（读侧）。
type ExpRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]ExpRecordItem, int64, error)
}

// LoginRecordQuery 登录记录查询端口（读侧）。
type LoginRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]LoginRecordItem, int64, error)
}

// CreatorMetricStore 创作中心指标存储端口（Redis 哈希按天累加）。
//
// Redis key 格式属于外部契约，由 infrastructure 适配器保持与原实现一致。
type CreatorMetricStore interface {
	// Available 报告底层存储是否可用；不可用时返回与原实现一致的错误。
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

// BanNotifier 封禁状态变更通知端口。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现，实现跨上下文协调。
type BanNotifier interface {
	// NotifyBanStatus 向被处理用户发送系统通知。
	NotifyBanStatus(ctx context.Context, receiverID uint, actionUserID uint, actionUserName, actionUserAvatar, title, content string)
}

// UserStatusReader 用户状态读取端口，供其它上下文（如中间件封禁校验）复用。
type UserStatusReader interface {
	CachedStatus(ctx context.Context, userID uint) (enum.UserStatus, error)
}
