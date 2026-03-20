package repository

import (
	"context"

	identityentity "f_ccg_video_back_ddd/internal/domain/identity/entity"
)

// UserAccountRepository 用户账号仓储：注册、账号查找、凭据与资料写入。
//
// 与 UserRepository 同由 infrastructure/persistence/identity.UserRepository 实现；
// 拆成两个接口是为了让只读状态的调用方（中间件、封禁流程）不被迫依赖写能力。
type UserAccountRepository interface {
	// RotateRefreshToken conditionally replaces a refresh digest, tied to the current password.
	RotateRefreshToken(ctx context.Context, userID uint, passwordHash, oldHash, newHash string) (bool, error)
	// SetRefreshToken sets the current refresh digest if the password has not changed.
	SetRefreshToken(ctx context.Context, userID uint, passwordHash, tokenHash string) (bool, error)

	// FindByID 按主键读取用户；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, userID uint) (*identityentity.User, error)
	// FindWithConfByID 一次查询读取用户及其一对一配置；用户不存在时返回 (nil, nil, nil)。
	FindWithConfByID(ctx context.Context, userID uint) (*identityentity.User, *identityentity.UserConf, error)
	// ExistsByUsernameOrEmail 判断用户名或邮箱是否已被占用。
	ExistsByUsernameOrEmail(ctx context.Context, username, email string) (bool, error)
	// FindByUsernameOrEmail 按「用户名或邮箱」登录标识读取用户；不存在时返回 (nil, nil)。
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*identityentity.User, error)
	// FindByEmail 按邮箱读取用户；不存在时返回 (nil, nil)。
	FindByEmail(ctx context.Context, email string) (*identityentity.User, error)
	// FindByOpenIDAndSource 按第三方 openID + 注册来源读取用户；不存在时返回 (nil, nil)。
	FindByOpenIDAndSource(ctx context.Context, openID, registerSource string) (*identityentity.User, error)
	// CountByUsernameExcluding 统计除 excludeUserID 之外使用该用户名的账号数。
	CountByUsernameExcluding(ctx context.Context, username string, excludeUserID uint) (int64, error)
	// CountByEmailExcluding 统计除 excludeUserID 之外使用该邮箱的账号数。
	CountByEmailExcluding(ctx context.Context, email string, excludeUserID uint) (int64, error)

	// Create 创建用户并写入 user_roles 关联，回填 user.ID。
	// 要求在事务内调用，并在同一事务中调用 InitializeDefaults。
	Create(ctx context.Context, user *identityentity.User, roleID uint) error
	// InitializeDefaults creates user configuration and default folder in the caller transaction.
	InitializeDefaults(ctx context.Context, userID uint) error
	// DeleteByID 删除用户（Casbin 绑定失败后的补偿路径）。
	DeleteByID(ctx context.Context, userID uint) error

	// UpdatePassword 覆盖写密码哈希。
	UpdatePassword(ctx context.Context, userID uint, passwordHash string) error
	// UpdateEmail 覆盖写邮箱。
	UpdateEmail(ctx context.Context, userID uint, email string) error
	// UpdateCurrentRole 更新当前角色 ID。
	UpdateCurrentRole(ctx context.Context, userID, roleID uint) error
	// UpdateProfile 部分更新用户资料。
	UpdateProfile(ctx context.Context, userID uint, changes identityentity.ProfileChanges) error
}

// UserConfRepository 用户配置写侧仓储。
type UserConfRepository interface {
	// FindByUserID 读取用户配置；不存在时返回 (nil, nil)。
	FindByUserID(ctx context.Context, userID uint) (*identityentity.UserConf, error)
	// Update 部分更新用户配置。
	Update(ctx context.Context, userID uint, changes identityentity.UserConfChanges) error
	// ReassignBanner 把所有使用 fromBannerID 的用户配置改为 toBannerID
	// （管理端更换注册默认横幅时的批量迁移）。
	ReassignBanner(ctx context.Context, fromBannerID, toBannerID uint) error
}
