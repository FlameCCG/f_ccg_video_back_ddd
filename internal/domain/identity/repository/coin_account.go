package repository

import (
	"context"
	identityentity "f_ccg_video_back_ddd/internal/domain/identity/entity"
)

// CoinAccountRepository 映射现有 users 表；所有方法在调用方事务内使用。
type CoinAccountRepository interface {
	// LockAccounts 按 ID 升序加锁，忽略重复与零 ID。
	LockAccounts(context.Context, []uint) error
	// FindForUpdate 锁定并重建账户；不存在时保留现有持久化错误契约。
	FindForUpdate(context.Context, uint) (*identityentity.CoinAccount, error)
	// SaveBalance 只保存余额，与流水复用同一事务。
	SaveBalance(context.Context, *identityentity.CoinAccount) error
}
