package rbac

import (
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
)

// AssignPermissionParams 分配/移除角色权限用例参数。
type AssignPermissionParams struct {
	RoleName    string
	Permissions []rbacvo.PermissionItem
}

// UpdateRolePermissionsParams 替换角色直接权限用例参数。
type UpdateRolePermissionsParams struct {
	RoleID           uint
	Permissions      []rbacvo.PermissionItem
	BreakInheritance bool
}

// RolePermissionSource 权限来源。
type RolePermissionSource string

const (
	// RolePermissionSourceDirect 角色自身直接绑定的策略
	RolePermissionSourceDirect RolePermissionSource = "direct"
	// RolePermissionSourceInheritDirect 来自直接父角色
	RolePermissionSourceInheritDirect RolePermissionSource = "inherit_direct"
	// RolePermissionSourceInheritIndirect 来自间接祖先角色
	RolePermissionSourceInheritIndirect RolePermissionSource = "inherit_indirect"
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
