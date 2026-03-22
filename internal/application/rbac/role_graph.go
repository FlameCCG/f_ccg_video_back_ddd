package rbac

import (
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// RoleGraph 通过注入的 Enforcer 端口查询角色继承关系。
type RoleGraph struct {
	enforcer    Enforcer
	permissions rbacdomain.PermissionEvaluator
}

// NewRoleGraph 构造角色继承关系查询器。
var _ rbacdomain.InheritanceFacts = (*RoleGraph)(nil)

func NewRoleGraph(enforcer Enforcer) *RoleGraph {
	return &RoleGraph{enforcer: enforcer, permissions: rbacdomain.PermissionEvaluator{}}
}

// DirectParents 获取角色的直接父角色名列表。
func (g *RoleGraph) DirectParents(roleName string) ([]string, error) {
	return g.enforcer.GetRolesForUser(roleName)
}

// ImplicitParents 获取角色的全部隐式父角色（含间接，不含自身）。
func (g *RoleGraph) ImplicitParents(roleName string) ([]string, error) {
	return g.enforcer.GetImplicitRolesForUser(roleName)
}

// DirectPermissionItems 获取角色直接权限列表。
func (g *RoleGraph) DirectPermissionItems(roleName string) ([]rbacdomain.PermissionItem, error) {
	rows, err := g.enforcer.GetPermissionsForUser(roleName)
	if err != nil {
		return nil, err
	}

	return g.permissions.NormalizeCasbinPolicyRows(rows), nil
}

// EffectivePermissionItems 获取角色有效权限列表（直接 + 继承）。
func (g *RoleGraph) EffectivePermissionItems(roleName string) ([]rbacdomain.PermissionItem, error) {
	rows, err := g.enforcer.GetImplicitPermissionsForUser(roleName)
	if err != nil {
		return nil, err
	}

	return g.permissions.NormalizeCasbinPolicyRows(rows), nil
}
