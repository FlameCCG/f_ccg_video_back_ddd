package identity

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// UserRepository 用户聚合的写侧仓储（接口所有权归领域层）。
type UserRepository interface {
	// FindByID 按主键读取用户；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, userID uint) (*User, error)
	// FindStatusByID 只读状态与用户名，供封禁流程使用；不存在时返回 (nil, nil)。
	FindStatusByID(ctx context.Context, userID uint) (*User, error)
	// FindProfileByID 只读 id/username/avatar，供通知渲染使用；不存在时返回 (nil, nil)。
	FindProfileByID(ctx context.Context, userID uint) (*User, error)
	// UsernameByID 读取用户名；用户不存在时返回空串。
	UsernameByID(ctx context.Context, userID uint) (string, error)
	// LockAccounts 在调用方事务中按 ID 升序锁定账户，忽略重复及零 ID。
	LockAccounts(ctx context.Context, userIDs []uint) error
	// UpdateStatus 更新用户状态。
	UpdateStatus(ctx context.Context, userID uint, status enum.UserStatus) error
	// ActivateIfBanned 仅当用户仍处于限时封禁态时置为正常，返回是否更新成功。
	ActivateIfBanned(ctx context.Context, userID uint) (bool, error)
	// CoinBalanceForUpdate 以行锁读取硬币余额（要求在事务内调用）。
	CoinBalanceForUpdate(ctx context.Context, userID uint) (float64, error)
	// CoinBalance 读取硬币余额（无锁）。
	CoinBalance(ctx context.Context, userID uint) (float64, error)
	// UpdateCoinBalance 覆盖写硬币余额。
	UpdateCoinBalance(ctx context.Context, userID uint, coins float64) error
	// ExpAndLevelForUpdate 以行锁读取经验与等级（要求在事务内调用）。
	ExpAndLevelForUpdate(ctx context.Context, userID uint) (exp int64, level int, err error)
	// UpdateExpAndLevel 覆盖写经验与等级。
	UpdateExpAndLevel(ctx context.Context, userID uint, exp int64, level int) error
	// UpdateLastLogin 更新最后登录 IP 与归属地。
	UpdateLastLogin(ctx context.Context, userID uint, ip, addr string) error
}

// ExpRecordRepository 经验流水仓储。
type ExpRecordRepository interface {
	// Create 写入经验流水；dedupKey 命中唯一索引时返回 ErrDuplicateRecord。
	Create(ctx context.Context, userID uint, delta int64, reason string, dedupKey *string) error
	// SumDailyGained 统计指定自然日内的正向经验总量。
	SumDailyGained(ctx context.Context, userID uint, now time.Time) (int64, error)
}

// CoinRecordRepository 硬币流水仓储。
type CoinRecordRepository interface {
	HasDedupKey(context.Context, uint, string) (bool, error)
	// Create 写入硬币流水；dedupKey 命中唯一索引时返回 ErrDuplicateRecord。
	Create(ctx context.Context, userID uint, delta float64, reason string, dedupKey *string) error
}

// BanRecordRepository 封禁记录仓储。
type BanRecordRepository interface {
	// Create 写入封禁记录，并回填自增 ID。
	Create(ctx context.Context, record *BanRecord) error
	// FindLatestByUserID 读取用户最近一条未解封封禁记录；无记录时返回 (nil, nil)。
	FindLatestByUserID(ctx context.Context, userID uint) (*BanRecord, error)
	// FindLatestUnreleasedForUpdate 锁定并读取指定状态中最近一条未解封记录。
	FindLatestUnreleasedForUpdate(
		ctx context.Context,
		userID uint,
		statuses []enum.UserStatus,
	) (*BanRecord, error)
	// MarkUnbanned 标记原封禁记录已解封。
	MarkUnbanned(ctx context.Context, recordID uint) error
}

// LoginRecordRepository 登录记录仓储。
type LoginRecordRepository interface {
	// Create 写入登录记录。
	Create(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) error
}

// UserStatusCache 用户状态缓存端口（Redis cache-aside + generation CAS）。
type UserStatusCache interface {
	// Load 读取缓存中的用户状态；未命中返回 (0, false)。
	Load(ctx context.Context, userID uint) (enum.UserStatus, bool)
	// Generation 读取当前 generation 快照（读库前捕获，用于 CAS 回填）。
	Generation(ctx context.Context, userID uint) int64
	// StoreIfGenerationMatch 仅在 generation 未变化时回填状态。
	StoreIfGenerationMatch(ctx context.Context, userID uint, status enum.UserStatus, expectedGen int64)
	// Write 权威写穿：直接写入最新状态并 bump generation。
	Write(userID uint, status enum.UserStatus)
	// Invalidate 删除缓存并 bump generation。
	Invalidate(userID uint)
}
