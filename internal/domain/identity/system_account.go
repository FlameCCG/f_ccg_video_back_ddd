package identity

import "context"

// NewSystemUser 根据 CLI 建号的邮箱、来源和初始权限构造系统用户，不执行普通注册校验。
func NewSystemUser(username, passwordHash string, roleID uint) *User {
	user := &User{username: username, email: username + "@example.com", registerSource: "system_flag"}
	user.ProvisionAdministrator(passwordHash, roleID)
	return user
}

// ProvisionAdministrator 是显式的初始化动作；保留强制改密和既有 6 级、20000 经验。
// 已有用户的余额、资料和封禁状态不参与此动作。
func (u *User) ProvisionAdministrator(passwordHash string, roleID uint) {
	u.password = passwordHash
	u.refreshTokenHash = ""
	u.mustChangePassword = true
	u.exp = 20000
	u.level = (ExperiencePolicy{}).Level(u.exp)
	u.currentRoleID = roleID
}

// SystemAccountRepository 是一次性初始化账号的领域持久化契约。
type SystemAccountRepository interface {
	FindByUsernameForUpdate(ctx context.Context, username string) (*User, error)
	CreateSystemAccount(ctx context.Context, user *User) (*User, error)
	SaveSystemAccount(ctx context.Context, user *User) error
	InitializeDefaults(ctx context.Context, userID uint) error
}
