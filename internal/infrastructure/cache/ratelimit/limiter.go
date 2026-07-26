// Package ratelimit 提供基于 Redis 的固定窗口计数限流（原 pkg/ratelimit）。
package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// allowScript 原子 INCR + 首次 EXPIRE，并自愈无 TTL 的 key（PTTL=-1）。
var allowScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local windowMs = tonumber(ARGV[2])

local count = redis.call("INCR", key)
if count == 1 then
  redis.call("PEXPIRE", key, windowMs)
else
  local ttl = redis.call("PTTL", key)
  if ttl < 0 then
    redis.call("PEXPIRE", key, windowMs)
  end
end

if count <= limit then
  return 1
end
return 0
`)

// Limiter 固定窗口限流器。
//
// 原实现为 pkg/ratelimit.Allow 包级函数直读 global.RedisClient；
// 新架构改为注入 Redis 客户端的实例。
type Limiter struct {
	redis *redis.Client
}

// New 构造限流器。
func New(redisClient *redis.Client) *Limiter {
	return &Limiter{redis: redisClient}
}

// Allow increments a Redis counter and reports whether it is still within limit.
// Redis 缺失或参数非法时放行（与原实现一致）。
func (l *Limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if l == nil || l.redis == nil || key == "" || limit <= 0 || window <= 0 {
		return true, nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	windowMs := window.Milliseconds()
	if windowMs <= 0 {
		windowMs = 1
	}

	allowed, err := allowScript.Run(ctx, l.redis, []string{key}, limit, windowMs).Int()
	if err != nil {
		return false, err
	}

	return allowed == 1, nil
}
