package entity

import (
	"strings"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
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
		return RoleRevision{}, rbacerrors.ErrRoleNotFound
	}
	var change RoleRevision
	if name != nil {
		value := strings.TrimSpace(*name)
		if value == "" {
			return change, rbacerrors.ErrRoleNameEmpty
		}
		if value != r.Name {
			change.Name = &value
		}
	}
	if desc != nil {
		value := strings.TrimSpace(*desc)
		change.Desc = &value
	}
	if change.Name == nil && change.Desc == nil {
		return change, rbacerrors.ErrNoUpdatableFields
	}
	if change.Name != nil {
		r.Name = *change.Name
	}
	if change.Desc != nil {
		r.Desc = *change.Desc
	}
	return change, nil
}

// EnsureDeletable 根据调用方取得的使用人数判断角色是否允许删除。
func (r *Role) EnsureDeletable(userCount int64) error {
	if r == nil {
		return rbacerrors.ErrRoleNotFound
	}
	if userCount > 0 {
		return rbacerrors.ErrRoleHasUsers
	}
	return nil
}

// Normalize 规范化字段，先校验图标安全，再校验标题非空。
func (u MenuFieldUpdate) Normalize() (MenuFieldUpdate, error) {
	if u.Icon != nil {
		if err := rbacvo.ValidateMenuIcon(strings.TrimSpace(*u.Icon)); err != nil {
			return MenuFieldUpdate{}, err
		}
	}
	if u.Title != nil && strings.TrimSpace(*u.Title) == "" {
		return MenuFieldUpdate{}, rbacerrors.ErrMenuTitleEmpty
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
		return MenuFieldUpdate{}, rbacerrors.ErrMenuNotFound
	}
	change, err := update.Normalize()
	if err != nil {
		return MenuFieldUpdate{}, err
	}
	if change.Title != nil {
		m.Title = *change.Title
	}
	if change.TitleEn != nil {
		m.TitleEn = *change.TitleEn
	}
	if change.TitleJa != nil {
		m.TitleJa = *change.TitleJa
	}
	if change.Icon != nil {
		m.Icon = *change.Icon
	}
	if change.Path != nil {
		m.Path = *change.Path
	}
	if change.Name != nil {
		m.Name = *change.Name
	}
	if change.Component != nil {
		m.Component = *change.Component
	}
	if change.KeepAlive != nil {
		m.KeepAlive = *change.KeepAlive
	}
	if change.ClearParent {
		m.ParentID = nil
	} else if change.ParentID != nil {
		value := *change.ParentID
		m.ParentID = &value
	}
	if change.SortOrder != nil {
		m.SortOrder = *change.SortOrder
	}
	return change, nil
}

// EnsureDeletable 有子菜单的菜单必须先清理子菜单。
func (m *Menu) EnsureDeletable(childCount int64) error {
	if m == nil {
		return rbacerrors.ErrMenuNotFound
	}
	if childCount > 0 {
		return rbacerrors.ErrDeleteChildMenusFirst
	}
	return nil
}
