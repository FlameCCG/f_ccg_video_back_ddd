package rbac

import "github.com/google/wire"

// ProviderSet 汇总 RBAC 上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewInheritService,
	NewInitService,
	NewMenuService,
	NewPermissionService,
	NewResourceSyncService,
	NewRoleGraph,
	NewRoleService,
)
