// Package rbac 编排角色、菜单、API 权限与继承关系管理。
//
// 先读 ports.go 的授权引擎端口，再读 permission_service.go、inherit_service.go 和 role_graph.go。
// 领域表达继承和权限覆盖规则，Casbin 策略读写由适配器实现；菜单是展示结构，API 鉴权仍需服务端策略。
// resource_sync_service.go 以实际路由清单同步 API 资源；menu_generation.go 对 AI 候选再次校验后才进入写入流程。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package rbac

import (
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// CreateRoleParams 创建角色用例参数。
type CreateRoleParams struct {
	Name        string
	Desc        string
	MenuIDs     []uint
	Permissions []domain.PermissionItem
	CopyRoleID  uint
}

// UpdateRoleParams 更新角色用例参数（指针为 nil 表示该字段不更新）。
type UpdateRoleParams struct {
	RoleID uint
	Name   *string
	Desc   *string
}

// CreateMenuParams 创建菜单用例参数。
type CreateMenuParams struct {
	Title     string
	TitleEn   string
	TitleJa   string
	Icon      string
	Path      string
	Name      string
	Component string
	KeepAlive bool
	ParentID  *uint
	SortOrder uint
	CreatorID uint
}

// UpdateMenuParams 更新菜单用例参数（指针为 nil 表示该字段不更新）。
type UpdateMenuParams struct {
	ID        uint
	Title     *string
	TitleEn   *string
	TitleJa   *string
	Icon      *string
	Path      *string
	Name      *string
	Component *string
	KeepAlive *bool
	ParentID  *uint
	// ParentProvided 区分「没有提交 parentId」与「提交了 parentId」。
	ParentProvided bool
	SortOrder      *uint
}

// AssignPermissionParams 分配/移除角色权限用例参数。
type AssignPermissionParams struct {
	RoleName    string
	Permissions []domain.PermissionItem
}

// UpdateRolePermissionsParams 替换角色直接权限用例参数。
type UpdateRolePermissionsParams struct {
	RoleID           uint
	Permissions      []domain.PermissionItem
	BreakInheritance bool
}

// RoleMenusResp 角色菜单详情响应（直连 + 继承，各自成树）。
type RoleMenusResp struct {
	DirectMenus  []*domain.Menu `json:"directMenus"`
	InheritMenus []*domain.Menu `json:"inheritMenus"`
}

// RolePermissionSource 权限来源。
type RolePermissionSource string

const (
	// RolePermissionSourceDirect 角色自身直接绑定的策略.
	RolePermissionSourceDirect RolePermissionSource = "direct"
	// RolePermissionSourceInheritDirect 来自直接父角色.
	RolePermissionSourceInheritDirect RolePermissionSource = "inherit_direct"
	// RolePermissionSourceInheritIndirect 来自间接祖先角色.
	RolePermissionSourceInheritIndirect RolePermissionSource = "inherit_indirect"
	// RolePermissionSourceInherited 兼容旧值（等同 inherit 类）.
	RolePermissionSourceInherited RolePermissionSource = "inherited"
)

// EffectivePermissionItem 角色有效权限项（含来源）。
type EffectivePermissionItem struct {
	Resource string               `json:"resource"`
	Action   string               `json:"action"`
	Source   RolePermissionSource `json:"source"`
	// Owner 直接持有该 p 策略的角色名
	Owner string `json:"owner,omitempty"`
	// TopParent 若要取消该继承权限，应解除的直接父角色名（仅继承类权限有值）
	TopParent string `json:"topParent,omitempty"`
}

// RolePermissionsResp 角色权限详情响应。
type RolePermissionsResp struct {
	// Direct 角色直接绑定的策略，Casbin 格式: [[role, path, method], ...]
	Direct [][]string `json:"direct"`
	// Effective 有效权限（直接 + 继承），含来源标记
	Effective []EffectivePermissionItem `json:"effective"`
	// Covered 被有效权限覆盖的具体 API key，格式 "METHOD:path"
	Covered []string `json:"covered"`
}

// RoleIndirectInheritItem 间接继承的角色及其顶层直接父。
type RoleIndirectInheritItem struct {
	// Name 间接继承到的角色名
	Name string `json:"name"`
	// Via 需要解绑的顶层直接父角色名
	Via string `json:"via"`
}

// RoleInheritsResp 角色继承详情：直接 + 间接。
type RoleInheritsResp struct {
	Direct   []string                  `json:"direct"`
	Indirect []RoleIndirectInheritItem `json:"indirect"`
}

// WildcardPermissionItem 系统中已存在的通配符权限（去重后）。
type WildcardPermissionItem struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// UserPermissionItem 用户最终权限项。
type UserPermissionItem struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// APIResourceItem 可用于权限配置的接口资源项。
type APIResourceItem struct {
	Path    string   `json:"path"`
	Method  string   `json:"method"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags,omitempty"`
}

// ResourceSyncResult 接口资源同步结果（原 service/rbac_service.ResourceSyncResult）。
type ResourceSyncResult struct {
	Total   int
	Created int
	Updated int
	Deleted int
}
