package rbac

// RoleAssignmentPolicy 决定直连角色集合的追加、移除和替换，应用层负责事务与 Casbin 协调。
type RoleAssignmentPolicy struct{}

func (RoleAssignmentPolicy) Plan(mode string, current, selected []*Role, requested int) ([]*Role, error) {
	if len(selected) != requested {
		return nil, ErrSomeRolesNotFound
	}
	switch mode {
	case "assign":
		result := append([]*Role(nil), current...)
		for _, role := range selected {
			for _, old := range current {
				if role.ID() == old.ID() {
					return nil, DuplicateRoleAssignedError(role.Name())
				}
			}
			result = append(result, role)
		}
		return result, nil
	case "remove":
		remove := make(map[uint]bool, len(selected))
		for _, role := range selected {
			remove[role.ID()] = true
		}
		var result []*Role
		for _, role := range current {
			if !remove[role.ID()] {
				result = append(result, role)
			}
		}
		return result, nil
	default:
		return selected, nil
	}
}

func (RoleAssignmentPolicy) EnsureMenuSelection(found, requested int) error {
	if found != requested {
		return ErrSomeMenusNotFound
	}
	return nil
}

func (r *Role) EnsureInitialMenus(found, requested int) error {
	if found != requested {
		return ErrPartialMenusNotFound
	}
	return nil
}
