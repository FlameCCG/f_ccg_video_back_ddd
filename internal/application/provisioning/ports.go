// Package provisioning 是运维初始化用例（原 flags 包中的 -su / -br 任务）。
//
// 这些用例只在 CLI 一次性任务中执行，不参与 HTTP 请求链路。
package provisioning

import (
	"context"
	"io"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// SuperUserRepository 超级管理员初始化所需的持久化端口。
type SuperUserRepository interface {
	// EnsureAdminRole 读取或创建管理员角色，返回角色 ID 与名称。
	EnsureAdminRole(ctx context.Context) (roleID uint, roleName string, created bool, err error)
	// UpsertSuperUser 创建或更新超级管理员账号，返回用户 ID 与是否为新建。
	UpsertSuperUser(ctx context.Context, username, passwordHash string, roleID uint) (userID uint, created bool, err error)
	// ReplaceUserRoles 用给定角色覆盖用户的角色关联。
	ReplaceUserRoles(ctx context.Context, userID, roleID uint) error
}

// CasbinRoleBinder Casbin 角色绑定端口。
type CasbinRoleBinder interface {
	// BindUserRole 清空用户旧角色后绑定新角色，并持久化策略。
	BindUserRole(userID uint, roleName string) error
}

// StorageDriver 对象存储端口（由 media 上下文的存储驱动实现）。
//
// 方法签名与原 service/storage_service 的驱动接口一致，
// Banner 初始化任务据此上传图片并拿到公开访问 URL。
type StorageDriver interface {
	SaveFile(ctx context.Context, path string, size int64, reader io.Reader, contentType string) (string, error)
}

// BannerRepository Banner 初始化所需的持久化端口。
type BannerRepository interface {
	// MigrateBannerTable 确保 banner 表已存在。
	MigrateBannerTable(ctx context.Context) error
	// UpsertBannerByCover 按封面 URL 幂等写入横幅：不存在则新建，存在则更新类型并置为显示。
	// 返回 true 表示新建。
	UpsertBannerByCover(ctx context.Context, coverURL string, bannerType enum.BannerType) (created bool, err error)
}
