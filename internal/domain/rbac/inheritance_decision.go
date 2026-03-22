package rbac

import "strings"

// InheritanceFacts 是按角色名读取关系事实的外部能力，不持有请求状态。
// 保留惰性查询，使已确定的直接继承不受无关祖先查询失败影响。
type InheritanceFacts interface {
	DirectParents(string) ([]string, error)
	ImplicitParents(string) ([]string, error)
}

// FindTopParentForOwner 查找「当前角色」为获得 owner 权限/角色，需要操作的直接父角色。
//   - owner 就是直接父：返回 owner + InheritKindDirect
//   - owner 经由某直接父间接获得：返回该直接父 + InheritKindIndirect
//   - owner 就是自己：InheritKindNone
func (p Inherit) FindTopParentForOwner(facts InheritanceFacts, roleName, ownerName string) (string, InheritKind, error) {
	roleName = strings.TrimSpace(roleName)
	ownerName = strings.TrimSpace(ownerName)

	if roleName == "" || ownerName == "" {
		return "", InheritKindNone, nil
	}

	if ownerName == roleName {
		return "", InheritKindNone, nil
	}

	direct, err := facts.DirectParents(roleName)
	if err != nil {
		return "", InheritKindNone, err
	}

	if p.ContainsRoleName(direct, ownerName) {
		return ownerName, InheritKindDirect, nil
	}

	// 间接：哪个直接父的隐式角色链包含 owner
	for _, parent := range direct {
		parent = strings.TrimSpace(parent)
		if parent == "" {
			continue
		}
		// 直接父自身就是链路中的一环
		if parent == ownerName {
			return parent, InheritKindDirect, nil
		}

		implicit, ierr := facts.ImplicitParents(parent)
		if ierr != nil {
			return "", InheritKindNone, ierr
		}

		if p.ContainsRoleName(implicit, ownerName) {
			return parent, InheritKindIndirect, nil
		}
	}

	return "", InheritKindNone, nil
}

// IsAlreadyInherited 当前角色是否已直接或间接继承 targetRole。
func (p Inherit) IsAlreadyInherited(facts InheritanceFacts, roleName, targetRole string) (bool, InheritKind, string, error) {
	direct, err := facts.DirectParents(roleName)
	if err != nil {
		return false, InheritKindNone, "", err
	}

	if p.ContainsRoleName(direct, targetRole) {
		return true, InheritKindDirect, targetRole, nil
	}

	implicit, err := facts.ImplicitParents(roleName)
	if err != nil {
		return false, InheritKindNone, "", err
	}

	if !p.ContainsRoleName(implicit, targetRole) {
		return false, InheritKindNone, "", nil
	}

	top, kind, err := p.FindTopParentForOwner(facts, roleName, targetRole)
	if err != nil {
		return true, InheritKindIndirect, "", err
	}

	if kind == InheritKindNone {
		kind = InheritKindIndirect
	}

	return true, kind, top, nil
}

// WouldCreateCycle 若 role 继承 parent，是否会形成环（parent 已直接/间接继承 role）。
//
// 检查的是反向可达关系：若 parent 已能到达 role，再添加 role → parent 就会闭环。
// “已经继承该角色”的重复边检查与“角色不能继承自身”的检查在调用用例中分别处理。
func (p Inherit) WouldCreateCycle(facts InheritanceFacts, roleName, parentName string) (bool, error) {
	// parent 已是 role 的祖先？由 IsAlreadyInherited 另判
	// role 已是 parent 的祖先？
	ok, _, _, err := p.IsAlreadyInherited(facts, parentName, roleName)

	return ok, err
}
