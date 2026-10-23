// Package cache 构造并关闭共享 Redis 客户端。
//
// 本目录只管理连接；各上下文的 key、TTL、Lua 和序列化放在对应子包。
// 客户端由 Wire 显式注入并在后台组件停止后关闭，避免运行中的回写或广播使用已关闭连接。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package cache

import (
	"context"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Redis 包装 *redis.Client，负责连接池生命周期。
type Redis struct {
	client *redis.Client
	logger *zap.Logger
}

// Client 返回底层 *redis.Client。
func (r *Redis) Client() *redis.Client {
	if r == nil {
		return nil
	}

	return r.client
}

// New 连接 Redis 并 Ping 探活；失败时 zap.Panic（与原 core.InitRedis 一致）。
func New(cfg *config.Redis, logger *zap.Logger) *Redis {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Host + ":" + cfg.Port,
		Password: cfg.Password,
		DB:       cfg.Db,
		PoolSize: cfg.PoolSize,
		// Redis 初始化是进程启动 root 场景，这些超时防止建连和池等待无限阻塞。
		DialTimeout:  ctxutil.RedisDialTimeout,
		ReadTimeout:  ctxutil.RedisReadTimeout,
		WriteTimeout: ctxutil.RedisWriteTimeout,
		PoolTimeout:  ctxutil.RedisPoolTimeout,
	})

	// Redis Ping 属于进程启动 root 场景，可以从 Background 派生独立健康检查超时。
	ctx, cancel := context.WithTimeout(context.Background(), ctxutil.RedisPingTimeout)
	defer cancel()

	res, err := rdb.Ping(ctx).Result()
	if err != nil {
		logger.Panic("redis连接失败", zap.Error(err))
	}

	logger.Info("redis连接成功", zap.String("res", res))

	return &Redis{client: rdb, logger: logger}
}

// Stop 关闭 Redis 客户端连接池。
func (r *Redis) Stop() {
	if r == nil || r.client == nil {
		return
	}

	if err := r.client.Close(); err != nil {
		r.logger.Warn("redis close failed", zap.Error(err))
	}

	r.client = nil

	r.logger.Info("redis连接已关闭")
}

// ProvideClient 从 Redis 包装中取出 *redis.Client 供仓储与限流器注入。
func ProvideClient(r *Redis) *redis.Client {
	return r.Client()
}

// Provide 供 wire 使用的 provider：返回 Redis 客户端与其资源关闭函数。
func Provide(cfg *config.Redis, logger *zap.Logger) (*Redis, func()) {
	client := New(cfg, logger)

	return client, client.Stop
}
