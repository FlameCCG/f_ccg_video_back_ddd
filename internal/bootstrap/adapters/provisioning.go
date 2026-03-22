package adapters

import (
	"context"
	contentopsapp "f_ccg_video_back_ddd/internal/application/contentops"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// SystemAccounts 只转换跨上下文调用，始终传播父初始化事务的 context。
type SystemAccounts struct {
	accounts *identityapp.SystemAccountService
	roles    *rbacapp.SystemRoleService
}

var _ provisioningapp.SystemAccounts = (*SystemAccounts)(nil)

func NewSystemAccounts(accounts *identityapp.SystemAccountService, roles *rbacapp.SystemRoleService) *SystemAccounts {
	return &SystemAccounts{accounts: accounts, roles: roles}
}
func (a *SystemAccounts) EnsureAdminRole(ctx context.Context) (uint, string, bool, error) {
	return a.roles.EnsureAdminRole(ctx)
}
func (a *SystemAccounts) UpsertSuperUser(ctx context.Context, username, passwordHash string, roleID uint) (uint, bool, error) {
	return a.accounts.UpsertSuperUser(ctx, username, passwordHash, roleID)
}
func (a *SystemAccounts) ReplaceUserRoles(ctx context.Context, userID, roleID uint) error {
	return a.roles.ReplaceUserRoles(ctx, userID, roleID)
}

type DefaultBanners struct {
	banners *contentopsapp.DefaultBannerService
	schema  provisioningapp.BannerTableMigrator
}

var _ provisioningapp.DefaultBanners = (*DefaultBanners)(nil)

func NewDefaultBanners(banners *contentopsapp.DefaultBannerService, schema provisioningapp.BannerTableMigrator) *DefaultBanners {
	return &DefaultBanners{banners: banners, schema: schema}
}
func (a *DefaultBanners) MigrateBannerTable(ctx context.Context) error {
	return a.schema.MigrateBannerTable(ctx)
}
func (a *DefaultBanners) UpsertBannerByCover(ctx context.Context, cover string, bannerType enum.BannerType) (bool, error) {
	return a.banners.UpsertBannerByCover(ctx, cover, bannerType)
}
