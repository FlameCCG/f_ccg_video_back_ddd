package identity

import (
	"context"
	"time"
)

// ProfileChanges 用户资料的部分更新集合；nil 字段表示本次不更新。
type ProfileChanges struct {
	Username    *string
	Avatar      *string
	Description *string
	Gender      *int
	Birthday    *time.Time
}

// IsEmpty 报告是否没有任何字段需要更新。
func (c ProfileChanges) IsEmpty() bool {
	return c.Username == nil &&
		c.Avatar == nil &&
		c.Description == nil &&
		c.Gender == nil &&
		c.Birthday == nil
}

// UserAccountRepository 用户账号仓储：注册、账号查找、凭据与资料写入。
//
// 与 UserRepository 同由 infrastructure/persistence/identity.UserRepository 实现；
// 拆成两个接口是为了让只读状态的调用方（中间件、封禁流程）不被迫依赖写能力。
type UserAccountRepository interface {
	// WithDefaultBanner 在 context 上附加「注册默认横幅 ID」，供用户创建钩子读取。
	//
	// 所有创建用户的路径（邮箱注册、5 家 OAuth 首次登录）都必须使用包装后的 ctx，
	// 否则新用户的 UserConf 会拿到兜底横幅而不是站点配置值（见 report.md FIND-002）。
	WithDefaultBanner(ctx context.Context) context.Context

	// FindByID 按主键读取用户；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, userID uint) (*User, error)
	// FindWithConfByID 一次查询读取用户及其一对一配置；用户不存在时返回 (nil, nil, nil)。
	FindWithConfByID(ctx context.Context, userID uint) (*User, *UserConf, error)
	// ExistsByUsernameOrEmail 判断用户名或邮箱是否已被占用。
	ExistsByUsernameOrEmail(ctx context.Context, username, email string) (bool, error)
	// FindByUsernameOrEmail 按「用户名或邮箱」登录标识读取用户；不存在时返回 (nil, nil)。
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*User, error)
	// FindByEmail 按邮箱读取用户；不存在时返回 (nil, nil)。
	FindByEmail(ctx context.Context, email string) (*User, error)
	// FindByOpenIDAndSource 按第三方 openID + 注册来源读取用户；不存在时返回 (nil, nil)。
	FindByOpenIDAndSource(ctx context.Context, openID, registerSource string) (*User, error)
	// CountByUsernameExcluding 统计除 excludeUserID 之外使用该用户名的账号数。
	CountByUsernameExcluding(ctx context.Context, username string, excludeUserID uint) (int64, error)
	// CountByEmailExcluding 统计除 excludeUserID 之外使用该邮箱的账号数。
	CountByEmailExcluding(ctx context.Context, email string, excludeUserID uint) (int64, error)

	// Create 创建用户并写入 user_roles 关联，回填 user.ID。
	// 要求在事务内调用；ctx 必须先经 WithDefaultBanner 包装。
	Create(ctx context.Context, user *User, roleID uint) error
	// DeleteByID 删除用户（Casbin 绑定失败后的补偿路径）。
	DeleteByID(ctx context.Context, userID uint) error

	// UpdatePassword 覆盖写密码哈希。
	UpdatePassword(ctx context.Context, userID uint, passwordHash string) error
	// UpdateEmail 覆盖写邮箱。
	UpdateEmail(ctx context.Context, userID uint, email string) error
	// UpdateCurrentRole 更新当前角色 ID。
	UpdateCurrentRole(ctx context.Context, userID, roleID uint) error
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
