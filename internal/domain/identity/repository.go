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
	FindStatusByIDForUpdate(ctx context.Context, userID uint) (*User, error)
	// FindProfileByID 只读 id/username/avatar，供通知渲染使用；不存在时返回 (nil, nil)。
	FindProfileByID(ctx context.Context, userID uint) (*User, error)
	// UsernameByID 读取用户名；用户不存在时返回空串。
	UsernameByID(ctx context.Context, userID uint) (string, error)
	// SaveStatus 只保存封禁/解封动作后的状态。
	SaveStatus(ctx context.Context, user *User) error
	// ActivateIfBanned 仅当用户仍处于限时封禁态时置为正常，返回是否更新成功。
	ActivateIfBanned(ctx context.Context, userID uint) (bool, error)
	// ExpAndLevelForUpdate 以行锁读取经验账户（要求在事务内调用）。
	ExpAndLevelForUpdate(ctx context.Context, userID uint) (*User, error)
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
	// Create 写入封禁记录，并返回含数据库生成 ID 与时间戳的记录。
	Create(ctx context.Context, record *BanRecord) (*BanRecord, error)
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
