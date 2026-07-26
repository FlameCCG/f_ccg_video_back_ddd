package rbac

import (
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// Enforcer Casbin 策略端口（基础设施实现，见 infrastructure/persistence/rbac/enforcer.go）。
//
// 应用层只依赖本接口，不 import casbin；方法集与原实现实际调用到的
// global.CasbinEnforcer 方法一一对应，语义（含 SavePolicy 全表重写）保持不变。
type Enforcer interface {
	// GetPermissionsForUser 读取主体的直接 p 策略。
	GetPermissionsForUser(user string) ([][]string, error)
	// GetImplicitPermissionsForUser 读取主体的有效 p 策略（含继承）。
	GetImplicitPermissionsForUser(user string) ([][]string, error)
	// GetPolicy 读取全部 p 策略。
	GetPolicy() ([][]string, error)
	// GetNamedGroupingPolicy 读取指定名称的分组策略（角色继承关系）。
	GetNamedGroupingPolicy(ptype string) ([][]string, error)
	// GetRolesForUser 读取主体的直接父角色。
	GetRolesForUser(name string) ([]string, error)
	// GetImplicitRolesForUser 读取主体的全部隐式父角色（含间接，不含自身）。
	GetImplicitRolesForUser(name string) ([]string, error)
	// GetUsersForRole 读取直接继承该角色的主体（用户或子角色）。
	GetUsersForRole(name string) ([]string, error)
	// AddPolicy 新增一条 p 策略。
	AddPolicy(sub, obj, act string) (bool, error)
	// RemovePolicy 移除一条 p 策略。
	RemovePolicy(sub, obj, act string) (bool, error)
	// RemoveFilteredPolicy 按字段过滤批量移除 p 策略。
	RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error)
	// AddRoleForUser 建立「user 继承 role」的分组策略。
	AddRoleForUser(user, role string) (bool, error)
	// DeleteRoleForUser 解除「user 继承 role」的分组策略。
	DeleteRoleForUser(user, role string) (bool, error)
	// DeleteRole 删除角色及其全部策略与分组关系。
	DeleteRole(role string) (bool, error)
	// SavePolicy 将内存策略整体落库。
	SavePolicy() error
}

// RouteLister 路由清单端口：返回 HTTP 引擎已注册的全部 method+path。
//
// 原 service/rbac_service/resource_sync.go 直接 import router 包调用 router.BuildEngine()，
// 造成 service → router 的反向依赖。新架构由应用层声明端口，
// 适配器在 interfaces 层（bootstrap 绑定），依赖方向恢复为 interfaces → application。
type RouteLister interface {
	// Routes 返回引擎注册的全部路由。
	Routes() []domain.RouteInfo
}

// ResourceMetaLoader OpenAPI 资源说明加载端口（文件 IO + YAML 解析属基础设施关注点）。
type ResourceMetaLoader interface {
	// Load 读取并解析 OpenAPI 文档，返回以「METHOD path」为键的说明表。
	// 失败时返回空表与错误，调用方按原实现降级为默认说明。
	Load() (map[string]domain.ResourceMeta, error)
}
