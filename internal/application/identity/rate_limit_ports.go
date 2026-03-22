package identity

import (
	"context"
	"time"
)

// SendLimitSubject 描述验证码发送的限流维度，Redis 键由适配器维护。
type SendLimitSubject struct {
	Type  int
	Scope SendLimitScope
	Value string
}

type SendLimitScope string

const (
	SendLimitEmail SendLimitScope = "email"
	SendLimitIP    SendLimitScope = "ip"
)

// RateLimiter 接收逻辑限流维度，不暴露缓存键。
type RateLimiter interface {
	Allow(ctx context.Context, subject SendLimitSubject, limit int64, window time.Duration) (bool, error)
}
