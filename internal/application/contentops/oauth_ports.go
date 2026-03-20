package contentops

import (
	"context"
)

// OAuthStateStore 一次性 OAuth state 存储端口。
//
// 绑定到 identity 上下文的 OAuth state 能力，Redis 优先、内存兜底，TTL 为 10 分钟。
type OAuthStateStore interface {
	// StoreOAuthState 保存一次性 OAuth state。
	StoreOAuthState(ctx context.Context, provider, state string) error
}
