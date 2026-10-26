// Package provisioning 实现超级管理员、角色绑定和默认横幅初始化。
//
// repository.go 实现 provisioning 用例要求的端口，复用默认用户上下文等已有持久化行为。
// 只负责一次性任务的存取细节，任务选择、步骤顺序与资源清理由 CLI 和 bootstrap 组织。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package provisioning

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/casbins"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/logctx"
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/provisioning"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 超级管理员账号的固定属性（与原 flags.FlagSuperUserInit 完全一致）。
const (
	superUserLevel = 6
	// superUserExp 确保经验值匹配 6 级，避免做任务/每日登录加经验时降为 1 级。
	superUserExp             = 20000
	superUserRegisterSource  = "system_flag"
	superUserEmailSuffix     = "@example.com"
	adminRoleDescription     = "管理员，拥有所有权限"
	adminRoleMissingWarnText = "超级管理员角色不存在，正在自动创建..."
)

// SuperUserRepository 超级管理员初始化的 GORM 实现。
type SuperUserRepository struct {
	db          *gorm.DB
	userRepoCtx UserContextDecorator
	logger      *zap.Logger
}

// UserContextDecorator 用于给创建用户的 ctx 附加默认横幅 ID
//
// 由 identity 的用户仓储实现，避免 CLI 建号路径与注册路径行为分叉。
type UserContextDecorator interface {
	WithDefaultBanner(ctx context.Context) context.Context
	InitializeDefaults(ctx context.Context, userID uint) error
}

var _ app.SuperUserRepository = (*SuperUserRepository)(nil)

// NewSuperUserRepository 构造超级管理员初始化仓储。
//
// userRepoCtx 传入 identity 的用户仓储，用于复用「默认横幅 ID 注入 ctx」的逻辑，
// 保证 CLI 创建的用户与注册流程一样会得到默认用户配置与默认收藏夹。
func NewSuperUserRepository(
	db *gorm.DB,
	userRepoCtx UserContextDecorator,
	logger *zap.Logger,
) *SuperUserRepository {
	return &SuperUserRepository{db: db, userRepoCtx: userRepoCtx, logger: logger}
}

// EnsureAdminRole 读取或创建管理员角色。
func (r *SuperUserRepository) EnsureAdminRole(ctx context.Context) (uint, string, bool, error) {
	db := gormx.DB(ctx, r.db)

	var adminRole po.Role
	if err := db.Where("name = ?", constants.AdminUserRole).First(&adminRole).Error; err == nil {
		return adminRole.ID, adminRole.Name, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, "", false, fmt.Errorf("查询管理员角色失败: %w", err)
	}

	logctx.With(ctx, r.logger).Warn(adminRoleMissingWarnText, zap.String("event", "persistence.repository.ensure_admin_role.degraded"), zap.String("operation", "persistence.repository.ensure_admin_role"), zap.String("role", constants.AdminUserRole))

	adminRole = po.Role{
		Name: constants.AdminUserRole,
		Desc: adminRoleDescription,
	}
	if err := db.Create(&adminRole).Error; err != nil {
		return 0, "", false, fmt.Errorf("创建管理员角色失败: %w", err)
	}

	return adminRole.ID, adminRole.Name, true, nil
}

// UpsertSuperUser 创建或更新超级管理员账号。
func (r *SuperUserRepository) UpsertSuperUser(
	ctx context.Context,
	username, passwordHash string,
	roleID uint,
) (uint, bool, error) {
	db := gormx.DB(ctx, r.db)

	var user po.User

	err := db.Where("username = ?", username).First(&user).Error
	if err == nil {
		// Update existing user
		user.Password = passwordHash
		user.MustChangePassword = true
		user.RefreshTokenHash = ""
		user.Level = superUserLevel
		user.Exp = superUserExp
		user.CurrentRoleID = roleID

		if saveErr := db.Save(&user).Error; saveErr != nil {
			return 0, false, fmt.Errorf("更新超级管理员用户密码失败: %w", saveErr)
		}

		return user.ID, false, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, fmt.Errorf("查询超级管理员用户失败: %w", err)
	}

	// Create new user（ctx 附带默认横幅 ID，行为与注册流程一致）
	createCtx := ctx
	if r.userRepoCtx != nil {
		createCtx = r.userRepoCtx.WithDefaultBanner(ctx)
	}

	user = po.User{
		Username:           username,
		Email:              username + superUserEmailSuffix,
		Password:           passwordHash,
		MustChangePassword: true,
		Level:              superUserLevel,
		Exp:                superUserExp,
		CurrentRoleID:      roleID,
		RegisterSource:     superUserRegisterSource,
	}
	if createErr := gormx.DB(createCtx, r.db).Create(&user).Error; createErr != nil {
		return 0, false, fmt.Errorf("创建超级管理员用户失败: %w", createErr)
	}

	if r.userRepoCtx != nil {
		if err := r.userRepoCtx.InitializeDefaults(createCtx, user.ID); err != nil {
			return 0, false, err
		}
	}

	return user.ID, true, nil
}

// ReplaceUserRoles 用给定角色覆盖用户的角色关联。
func (r *SuperUserRepository) ReplaceUserRoles(ctx context.Context, userID, roleID uint) error {
	db := gormx.DB(ctx, r.db)

	user := po.User{Model: po.Model{ID: userID}}
	role := po.Role{Model: po.Model{ID: roleID}}

	return db.Model(&user).Association("Roles").Replace([]*po.Role{&role})
}

// CasbinRoleBinder Casbin 角色绑定的实现。
type CasbinRoleBinder struct {
	enforcer *casbin.SyncedEnforcer
	logger   *zap.Logger
}

var _ app.CasbinRoleBinder = (*CasbinRoleBinder)(nil)

// NewCasbinRoleBinder 构造 Casbin 角色绑定器。
func NewCasbinRoleBinder(enforcer *casbin.SyncedEnforcer, logger *zap.Logger) *CasbinRoleBinder {
	return &CasbinRoleBinder{enforcer: enforcer, logger: logger}
}

// BindUserRole 清空用户旧角色后绑定新角色，并持久化策略。
func (b *CasbinRoleBinder) BindUserRole(userID uint, roleName string) error {
	if b.enforcer == nil {
		return errors.New("casbin enforcer 未初始化")
	}

	sub := casbins.GetUserSub(userID)
	// Clear old roles for user to avoid duplication
	_, _ = b.enforcer.DeleteRolesForUser(sub)

	if _, err := b.enforcer.AddRoleForUser(sub, roleName); err != nil {
		return fmt.Errorf("Casbin绑定角色失败: %w", err) //nolint:staticcheck // 保留原日志文案
	}

	if err := b.enforcer.SavePolicy(); err != nil {
		return fmt.Errorf("Casbin保存策略失败: %w", err) //nolint:staticcheck // 保留原日志文案
	}

	b.logger.Info("Casbin权限策略同步成功", zap.String("event", "persistence.repository.bind_user_role.diagnostic"), zap.String("operation", "persistence.repository.bind_user_role"), zap.String("sub", sub), zap.String("role", roleName))

	return nil
}

// BannerRepository Banner 初始化的 GORM 实现。
type BannerRepository struct {
	db *gorm.DB
}

var _ app.BannerRepository = (*BannerRepository)(nil)

// NewBannerRepository 构造 Banner 初始化仓储。
func NewBannerRepository(db *gorm.DB) *BannerRepository {
	return &BannerRepository{db: db}
}

// MigrateBannerTable 确保 banner 表已存在。
func (r *BannerRepository) MigrateBannerTable(ctx context.Context) error {
	return gormx.DB(ctx, r.db).AutoMigrate(&po.Banner{})
}

// UpsertBannerByCover 按封面 URL 幂等写入横幅。
func (r *BannerRepository) UpsertBannerByCover(
	ctx context.Context,
	coverURL string,
	bannerType enum.BannerType,
) (bool, error) {
	db := gormx.DB(ctx, r.db)

	var banner po.Banner
	if err := db.Where("cover = ?", coverURL).First(&banner).Error; err != nil {
		// Create new one（与原实现一致：任何查询失败都按「不存在」走新建分支）
		banner = po.Banner{
			Cover:       coverURL,
			Href:        "",
			Show:        true,
			Type:        bannerType,
			PartitionID: 0,
		}
		if createErr := db.Create(&banner).Error; createErr != nil {
			return false, createErr
		}

		return true, nil
	}

	// Update existing one
	banner.Type = bannerType
	banner.Show = true

	if err := db.Save(&banner).Error; err != nil {
		return false, err
	}

	return false, nil
}
