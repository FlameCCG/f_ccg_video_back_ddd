package contentops

import (
	"context"
	"time"
)

// SiteStatStore 站点流量统计存储端口（Redis 实现）。
//
// PV / UV / 在线人数的 Redis key 格式、TTL 与 5 分钟在线窗口属于外部契约。
type SiteStatStore interface {
	// Touch 站点打点：累加 PV、记录当日 UV、维护在线用户；
	// 第二个返回值表示该登录用户是否为当日首次打点。
	Touch(ctx context.Context, userID uint, ip string) (*TouchResult, bool, error)
	// Heartbeat 在线心跳：不累加 PV，只记录当日 UV 与在线用户。
	Heartbeat(ctx context.Context, userID uint, ip string) (*HeartbeatResult, error)
	// OnlineCount 读取当前在线人数（先清理过期成员）。
	OnlineCount(ctx context.Context) (int64, error)
	// DailyUV 读取指定自然日的访问 UV。
	DailyUV(ctx context.Context, day time.Time) (int64, error)
}

// LoginExpGranter 每日登录经验发放端口。
//
// 跨上下文依赖：应绑定到 identity 上下文的经验服务。
type LoginExpGranter interface {
	// LoginExp 发放当日登录经验（幂等由 DB 唯一键兜底）。
	LoginExp(ctx context.Context, userID uint) error
}

// LoginCoinGranter 每日登录硬币发放端口。
//
// 跨上下文依赖：应绑定到 identity 上下文的硬币服务。
type LoginCoinGranter interface {
	// LoginCoin 发放当日登录硬币（幂等由 DB 唯一键兜底）。
	LoginCoin(ctx context.Context, userID uint) error
}
