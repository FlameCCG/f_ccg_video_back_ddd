package rbac

// MenuGrantPolicy 判断新菜单授予创建者当前角色的资格；应用层依次取得用户与角色事实。
type MenuGrantPolicy struct{}

func (MenuGrantPolicy) EnsureCreator(creatorID uint) error {
	if creatorID == 0 {
		return ErrMenuCreatorRoleMissing
	}
	return nil
}

func (MenuGrantPolicy) EnsureCurrentRole(found bool, roleID uint) error {
	if !found || roleID == 0 {
		return ErrMenuCreatorRoleMissing
	}
	return nil
}
