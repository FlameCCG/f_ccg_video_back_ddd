package identity

import "context"

// UserStatusRefresher 在提交后重建权威状态缓存，使用 generation/CAS 防止旧读取回填。
type UserStatusRefresher interface {
	Refresh(ctx context.Context, userID uint) error
}
