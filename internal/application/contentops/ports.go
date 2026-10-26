package contentops

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// 端口阅读：运营页面需要的统计、分区事实、缓存、配置写回与登录奖励通过端口提供。
// ConfigFileWriter 负责原子替换文件；用例在 ConfigStore.Update 临界区内先持久化再发布。

// BannerCache 可见轮播图缓存端口（Redis 实现在 infrastructure/cache/contentops）。
//
// 原实现为 service/banner_service/cache.go 的一组包级函数 + global.RedisClient；
// 端口方法刻意不带 context：与原实现一致地使用固定 300ms 独立超时，
// 避免请求 context 取消时连缓存回填都做不完。Redis key 前缀与 TTL 属于外部契约。
type BannerCache interface {
	// LoadVisible 读取可见轮播图缓存；未命中或反序列化失败返回 (nil, false)。
	LoadVisible(bannerType enum.BannerType, partitionID *uint) ([]domain.Banner, bool)
	// StoreVisible 回填可见轮播图缓存（失败仅记录日志）。
	StoreVisible(bannerType enum.BannerType, partitionID *uint, items []domain.Banner)
	// InvalidateVisible 按前缀清空全部可见轮播图缓存。
	InvalidateVisible()
}

// VideoPartitionQuery 视频分区存在性查询端口。
//
// 跨上下文读侧投影：分区聚合归 video 上下文所有，这里只做一次 COUNT 校验，
// 按 decisions.md D-011 允许在本上下文的 infrastructure 中直读 video_partitions 表。
type VideoPartitionQuery interface {
	// Exists 报告分区是否存在。
	Exists(ctx context.Context, partitionID uint) (bool, error)
}

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

// AdminStatsQuery 管理端站点统计的读侧投影端口。
//
// 统计涉及 users / videos / comments / user_login_records 四张表，
// 分属 identity、video、comment 上下文；按 decisions.md D-011，
// 纯 COUNT 读侧投影允许在本上下文的 infrastructure 中直读，避免为计数引入跨上下文调用链。
type AdminStatsQuery interface {
	// CountUsers 统计用户总数。
	CountUsers(ctx context.Context) (int64, error)
	// CountPublishedVideos 统计已发布视频总数。
	CountPublishedVideos(ctx context.Context) (int64, error)
	// CountComments 统计评论总数。
	CountComments(ctx context.Context) (int64, error)
	// CountUsersCreatedBetween 统计 [start, end) 区间内注册的用户数。
	CountUsersCreatedBetween(ctx context.Context, start, end time.Time) (int64, error)
	// CountDistinctLoginUsersBetween 统计 [start, end) 区间内去重的登录用户数。
	CountDistinctLoginUsersBetween(ctx context.Context, start, end time.Time) (int64, error)
	// CountPublishedVideosCreatedBetween 统计 [start, end) 区间内创建的已发布视频数。
	CountPublishedVideosCreatedBetween(ctx context.Context, start, end time.Time) (int64, error)
}

// ConfigFileWriter 配置文件写回端口。
//
// 管理端更新配置后需要把对应配置段持久化回 YAML 文件（原 viper.WriteConfig）。
// 文件格式与序列化细节属于技术实现，落在 infrastructure；
// 实现须把读取失败与写入失败分别包装为 ErrConfigFileRead / ErrConfigFileWrite。
type ConfigFileWriter interface {
	// Write 把 item 写入配置文件的 name 配置段。
	Write(name string, item any) error
}

// OAuthStateStore 一次性 OAuth state 存储端口。
//
// 跨上下文依赖：应绑定到 identity 上下文的 OAuth state 能力
// （原 service/login_service.StoreOAuthState，Redis 优先、内存兜底，TTL 10 分钟）。
type OAuthStateStore interface {
	// StoreOAuthState 保存一次性 OAuth state。
	StoreOAuthState(ctx context.Context, provider, state string) error
}

// LoginExpGranter 每日登录经验发放端口。
//
// 跨上下文依赖：应绑定到 identity 上下文的经验服务（原 user_service.ExpService.LoginExp）。
type LoginExpGranter interface {
	// LoginExp 发放当日登录经验（幂等由 DB 唯一键兜底）。
	LoginExp(ctx context.Context, userID uint) error
}

// LoginCoinGranter 每日登录硬币发放端口。
//
// 跨上下文依赖：应绑定到 identity 上下文的硬币服务（原 user_service.CoinService.LoginCoin）。
type LoginCoinGranter interface {
	// LoginCoin 发放当日登录硬币（幂等由 DB 唯一键兜底）。
	LoginCoin(ctx context.Context, userID uint) error
}
