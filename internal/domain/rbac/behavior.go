package rbac

import (
	"strings"
)

// RoleRevision 是规范化后的角色更新，不包含 Casbin 策略或数据库列名。
type RoleRevision struct {
	Name *string
	Desc *string
}

// Revise 保护名称约束，并在全部校验成功后更新角色。
// 名称唯一性需要外部事实，由用例在持久化前另行确认。
func (r *Role) Revise(name, desc *string) (RoleRevision, error) {
	if r == nil {
		return RoleRevision{}, ErrRoleNotFound
	}
	var change RoleRevision
	if name != nil {
		value := strings.TrimSpace(*name)
		if value == "" {
			return change, ErrRoleNameEmpty
		}
		if value != r.name {
			change.Name = &value
		}
	}
	if desc != nil {
		value := strings.TrimSpace(*desc)
		change.Desc = &value
	}
	if change.Name == nil && change.Desc == nil {
		return change, ErrNoUpdatableFields
	}
	if change.Name != nil {
		r.name = *change.Name
	}
	if change.Desc != nil {
		r.desc = *change.Desc
	}
	return change, nil
}

// EnsureDeletable 根据调用方取得的使用人数判断角色是否允许删除。
func (r *Role) EnsureDeletable(userCount int64) error {
	if r == nil {
		return ErrRoleNotFound
	}
	if userCount > 0 {
		return ErrRoleHasUsers
	}
	return nil
}

// Normalize 规范化字段，先校验图标安全，再校验标题非空。
func (u MenuFieldUpdate) Normalize() (MenuFieldUpdate, error) {
	if u.Icon != nil {
		if err := MenuIcon(strings.TrimSpace(*u.Icon)).Validate(); err != nil {
			return MenuFieldUpdate{}, err
		}
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) == "" {
		return MenuFieldUpdate{}, ErrMenuTitleEmpty
	}
	for _, field := range []**string{&u.Title, &u.TitleEn, &u.TitleJa, &u.Icon, &u.Path, &u.Name, &u.Component} {
		if *field != nil {
			value := strings.TrimSpace(**field)
			*field = &value
		}
	}
	return u, nil
}

// Revise 将规范化和字段修改收敛到实体；树结构与同层排序由用例在事务内协调。
func (m *Menu) Revise(update MenuFieldUpdate) (MenuFieldUpdate, error) {
	if m == nil {
		return MenuFieldUpdate{}, ErrMenuNotFound
	}
	change, err := update.Normalize()
	if err != nil {
		return MenuFieldUpdate{}, err
	}
	if change.Title != nil {
		m.title = *change.Title
	}
	if change.TitleEn != nil {
		m.titleEn = *change.TitleEn
	}
	if change.TitleJa != nil {
		m.titleJa = *change.TitleJa
	}
	if change.Icon != nil {
		m.icon = *change.Icon
	}
	if change.Path != nil {
		m.path = *change.Path
	}
	if change.Name != nil {
		m.name = *change.Name
	}
	if change.Component != nil {
		m.component = *change.Component
	}
	if change.KeepAlive != nil {
		m.keepAlive = *change.KeepAlive
	}
	if change.ClearParent {
		m.parentID = nil
	} else if change.ParentID != nil {
		value := *change.ParentID
		m.parentID = &value
	}
	if change.SortOrder != nil {
		m.sortOrder = *change.SortOrder
	}
	return change, nil
}

// EnsureDeletable 有子菜单的菜单必须先清理子菜单。
func (m *Menu) EnsureDeletable(childCount int64) error {
	if m == nil {
		return ErrMenuNotFound
	}
	if childCount > 0 {
		return ErrDeleteChildMenusFirst
	}
	return nil
}
