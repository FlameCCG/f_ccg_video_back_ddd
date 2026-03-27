package identity

import (
	"context"
	"time"
)

// SessionStore 保存各登录会话的刷新凭据；缺失或过期的会话不得继续鉴权。
// Rotate 原子轮换摘要，并容忍短期重试；缺失会话返回 false，宽限期外重放返回
// ErrRefreshTokenReused 并撤销所属会话，不影响同用户的其他设备。
type SessionStore interface {
	Create(ctx context.Context, userID uint, sessionID, refreshHash string, expiresAt time.Time) error
	Exists(ctx context.Context, userID uint, sessionID string) (bool, error)
	Rotate(ctx context.Context, userID uint, sessionID, oldHash, newHash string, expiresAt time.Time) (bool, error)
	SnapshotUser(ctx context.Context, userID uint) ([]string, error)
	RevokeSessions(ctx context.Context, userID uint, sessionIDs []string) error
}
