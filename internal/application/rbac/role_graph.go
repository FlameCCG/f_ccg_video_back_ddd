package rbac

import (
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// RoleGraph 角色继承关系查询（原 api/rbac_api/role_inherit_helpers.go 中直连
// global.CasbinEnforcer 的一组辅助函数，改为经 Enforcer 端口注入）。
type RoleGraph struct {
	enforcer Enforcer
}

// NewRoleGraph 构造角色继承关系查询器。
func NewRoleGraph(enforcer Enforcer) *RoleGraph {
	return &RoleGraph{enforcer: enforcer}
}

// DirectParents 获取角色的直接父角色名列表。
func (g *RoleGraph) DirectParents(roleName string) ([]string, error) {
	return g.enforcer.GetRolesForUser(roleName)
}

// ImplicitParents 获取角色的全部隐式父角色（含间接，不含自身）。
func (g *RoleGraph) ImplicitParents(roleName string) ([]string, error) {
	return g.enforcer.GetImplicitRolesForUser(roleName)
}

// FindTopParentForOwner 查找「当前角色」为获得 owner 权限/角色，需要操作的直接父角色。
//   - owner 就是直接父：返回 owner + InheritKindDirect
//   - owner 经由某直接父间接获得：返回该直接父 + InheritKindIndirect
//   - owner 就是自己：InheritKindNone
func (g *RoleGraph) FindTopParentForOwner(roleName, ownerName string) (string, domain.InheritKind, error) {
	roleName = strings.TrimSpace(roleName)
	ownerName = strings.TrimSpace(ownerName)

	if roleName == "" || ownerName == "" {
		return "", domain.InheritKindNone, nil
	}

	if ownerName == roleName {
		return "", domain.InheritKindNone, nil
	}

	direct, err := g.DirectParents(roleName)
	if err != nil {
		return "", domain.InheritKindNone, err
	}

	if domain.ContainsRoleName(direct, ownerName) {
		return ownerName, domain.InheritKindDirect, nil
	}

	// 间接：哪个直接父的隐式角色链包含 owner
	for _, parent := range direct {
		parent = strings.TrimSpace(parent)
		if parent == "" {
			continue
		}
		// 直接父自身就是链路中的一环
		if parent == ownerName {
			return parent, domain.InheritKindDirect, nil
		}

		implicit, ierr := g.ImplicitParents(parent)
		if ierr != nil {
			return "", domain.InheritKindNone, ierr
		}

		if domain.ContainsRoleName(implicit, ownerName) {
			return parent, domain.InheritKindIndirect, nil
		}
	}

	return "", domain.InheritKindNone, nil
}

// IsAlreadyInherited 当前角色是否已直接或间接继承 targetRole。
func (g *RoleGraph) IsAlreadyInherited(roleName, targetRole string) (bool, domain.InheritKind, string, error) {
	direct, err := g.DirectParents(roleName)
	if err != nil {
		return false, domain.InheritKindNone, "", err
	}

	if domain.ContainsRoleName(direct, targetRole) {
		return true, domain.InheritKindDirect, targetRole, nil
	}

	implicit, err := g.ImplicitParents(roleName)
	if err != nil {
		return false, domain.InheritKindNone, "", err
	}

	if !domain.ContainsRoleName(implicit, targetRole) {
		return false, domain.InheritKindNone, "", nil
	}

	top, kind, err := g.FindTopParentForOwner(roleName, targetRole)
	if err != nil {
		return true, domain.InheritKindIndirect, "", err
	}

	if kind == domain.InheritKindNone {
		kind = domain.InheritKindIndirect
	}

	return true, kind, top, nil
}

// WouldCreateCycle 若 role 继承 parent，是否会形成环（parent 已直接/间接继承 role）。
func (g *RoleGraph) WouldCreateCycle(roleName, parentName string) (bool, error) {
	// parent 已是 role 的祖先？由 IsAlreadyInherited 另判
	// role 已是 parent 的祖先？
	ok, _, _, err := g.IsAlreadyInherited(parentName, roleName)

	return ok, err
}

// DirectPermissionItems 获取角色直接权限列表。
func (g *RoleGraph) DirectPermissionItems(roleName string) ([]domain.PermissionItem, error) {
	rows, err := g.enforcer.GetPermissionsForUser(roleName)
	if err != nil {
		return nil, err
	}

	return domain.NormalizeCasbinPolicyRows(rows), nil
}

// EffectivePermissionItems 获取角色有效权限列表（直接 + 继承）。
func (g *RoleGraph) EffectivePermissionItems(roleName string) ([]domain.PermissionItem, error) {
	rows, err := g.enforcer.GetImplicitPermissionsForUser(roleName)
	if err != nil {
		return nil, err
	}

	return domain.NormalizeCasbinPolicyRows(rows), nil
}
