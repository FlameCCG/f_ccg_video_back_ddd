package identity

import (
	"context"
	"time"
)

// RateLimiter 固定窗口限流端口（实现为 infrastructure/cache/ratelimit.Limiter）。
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}
