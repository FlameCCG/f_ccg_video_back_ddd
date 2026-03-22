package identity

import (
	"context"
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
	FindByID(ctx context.Context, userID uint) (*User, error)
	// FindByIDForUpdate 在用例事务内锁定最新账号状态。
	FindByIDForUpdate(ctx context.Context, userID uint) (*User, error)
	// FindWithConfByID 一次查询读取用户及其一对一配置；用户不存在时返回 (nil, nil, nil)。
	FindWithConfByID(ctx context.Context, userID uint) (*User, *UserConf, error)
	// ExistsByUsernameOrEmail 判断用户名或邮箱是否已被占用。
	ExistsByUsernameOrEmail(ctx context.Context, username, email string) (bool, error)
	// FindByUsernameOrEmail 按「用户名或邮箱」登录标识读取用户；不存在时返回 (nil, nil)。
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*User, error)
	// FindByEmail 按邮箱读取用户；不存在时返回 (nil, nil)。
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByEmailForUpdate(ctx context.Context, email string) (*User, error)
	// FindByOpenIDAndSource 按第三方 openID + 注册来源读取用户；不存在时返回 (nil, nil)。
	FindByOpenIDAndSource(ctx context.Context, openID, registerSource string) (*User, error)
	// CountByUsernameExcluding 统计除 excludeUserID 之外使用该用户名的账号数。
	CountByUsernameExcluding(ctx context.Context, username string, excludeUserID uint) (int64, error)
	// CountByEmailExcluding 统计除 excludeUserID 之外使用该邮箱的账号数。
	CountByEmailExcluding(ctx context.Context, email string, excludeUserID uint) (int64, error)

	// Create 创建用户并写入 user_roles 关联，返回含数据库生成 ID 与时间戳的用户。
	// 要求在事务内调用，并在同一事务中调用 InitializeDefaults。
	Create(ctx context.Context, user *User, roleID uint) (*User, error)
	// InitializeDefaults 在调用方事务内创建身份配置，收藏夹经视频应用端口初始化。
	InitializeDefaults(ctx context.Context, userID uint) error
	// DeleteByID 删除用户（Casbin 绑定失败后的补偿路径）。
	DeleteByID(ctx context.Context, userID uint) error

	// SaveCredentials 仅保存密码、刷新凭据与强制改密标记。
	SaveCredentials(ctx context.Context, user *User) error
	// SaveEmail 仅保存经过绑定动作后的邮箱。
	SaveEmail(ctx context.Context, user *User) error
	// SaveCurrentRole 仅保存经过角色选择动作后的当前角色。
	SaveCurrentRole(ctx context.Context, user *User) error
	// UpdateProfile 部分更新用户资料。
	UpdateProfile(ctx context.Context, userID uint, changes ProfileChanges) error
}

// UserConfRepository 用户配置写侧仓储。
type UserConfRepository interface {
	// FindByUserID 读取用户配置；不存在时返回 (nil, nil)。
	FindByUserID(ctx context.Context, userID uint) (*UserConf, error)
	// Update 部分更新用户配置。
	Update(ctx context.Context, userID uint, changes UserConfChanges) error
	// ReassignBanner 把所有使用 fromBannerID 的用户配置改为 toBannerID
	// （管理端更换注册默认横幅时的批量迁移）。
	ReassignBanner(ctx context.Context, fromBannerID, toBannerID uint) error
}
