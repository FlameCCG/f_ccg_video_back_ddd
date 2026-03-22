// Package provisioning implements technical CLI migration and Casbin adapters.
package provisioning

import (
	"context"
	"errors"
	app "f_ccg_video_back_ddd/internal/application/provisioning"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/casbins"
	"fmt"
	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
		return errors.New("Casbin enforcer 未初始化")
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

	b.logger.Info("Casbin权限策略同步成功", zap.String("sub", sub), zap.String("role", roleName))

	return nil
}

// BannerSchema only migrates the existing table; state writes belong to contentops.
type BannerSchema struct{ db *gorm.DB }

var _ app.BannerTableMigrator = (*BannerSchema)(nil)

func NewBannerSchema(db *gorm.DB) *BannerSchema { return &BannerSchema{db: db} }
func (r *BannerSchema) MigrateBannerTable(ctx context.Context) error {
	return gormx.DB(ctx, r.db).AutoMigrate(&po.Banner{})
}
