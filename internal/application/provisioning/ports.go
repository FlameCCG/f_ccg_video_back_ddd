// Package provisioning 编排超级管理员和默认横幅的一次性初始化。
//
// superuser_service.go 与 banner_service.go 只在 CLI 任务中调用；-db 等任务的选择和依赖范围在 bootstrap/providers_cli.go。
// 初始化可能涉及数据库、角色绑定和存储等多个步骤；重复执行与中途失败的行为应按每个端口实现判断。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package provisioning

import (
	"context"
	"io"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// 端口阅读：一次性建号和横幅初始化经最小存取能力执行，CLI 任务本身不直接依赖数据库句柄。
// 角色引擎和对象存储具有独立失败点，重复执行规则应查看服务与适配器的实际约定。

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
