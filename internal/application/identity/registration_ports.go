package identity

import (
	"context"
)

// UserRoleBinder Casbin 用户-角色绑定端口（写侧）。
//
// 跨上下文依赖：应绑定到 rbac 上下文的 Casbin 能力。
type UserRoleBinder interface {
	// AddRoleForUser 为用户绑定角色；实现负责把 userID 转换为 Casbin subject。
	AddRoleForUser(userID uint, roleName string) error
}

// DefaultFolderEnsurer 默认收藏夹兜底创建端口。
//
// 跨上下文依赖：应绑定到 video 上下文的收藏夹服务。
type DefaultFolderEnsurer interface {
	EnsureDefaultFolder(ctx context.Context, userID uint) error
}

// DefaultFolderInitializer 在账号创建事务中初始化视频上下文的默认收藏夹，不触发缓存协调。
type DefaultFolderInitializer interface {
	InitializeDefaultFolder(ctx context.Context, userID uint) error
}
