package entity

import "time"

// Role 角色实体，是 RBAC 上下文的聚合根。
//
// JSON 输出 id/name/desc/menus；关联用户的存在性通过仓储计数判断，领域实体不持有 Users 关联。
type Role struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	Name      string    `json:"name"`
	Desc      string    `json:"desc"`
	Menus     []*Menu   `json:"menus,omitempty"`
}

// RoleMenu 角色-菜单关联（含「是否由继承而来」标记）。
type RoleMenu struct {
	RoleID      uint
	MenuID      uint
	IsInherited bool
}

// NewRole 构造角色实体。
func NewRole(name, desc string) *Role {
	return &Role{Name: name, Desc: desc}
}

// NewInheritedRoleMenus 为角色批量构造「继承而来」的菜单关联。
func NewInheritedRoleMenus(roleID uint, menuIDs []uint) []RoleMenu {
	items := make([]RoleMenu, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		items = append(items, RoleMenu{RoleID: roleID, MenuID: menuID, IsInherited: true})
	}

	return items
}

// NewDirectRoleMenus 为角色批量构造「直连」的菜单关联。
func NewDirectRoleMenus(roleID uint, menus []*Menu) []RoleMenu {
	items := make([]RoleMenu, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}

		items = append(items, RoleMenu{RoleID: roleID, MenuID: menu.ID, IsInherited: false})
	}

	return items
}

// DedupeMenuIDs 排除集合 exclude 中的菜单 ID 并去重，保持入参顺序。
func DedupeMenuIDs(menuIDs []uint, exclude map[uint]struct{}) []uint {
	filtered := make([]uint, 0, len(menuIDs))

	seen := make(map[uint]struct{}, len(menuIDs))
	for _, id := range menuIDs {
		if _, ok := exclude[id]; ok {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		filtered = append(filtered, id)
	}

	return filtered
}

// MergeMenuIDs 把模板角色菜单与显式指定的菜单 ID 合并为去重集合。
//
// 语义模板角色的菜单全量纳入，显式菜单仅纳入正数 ID。
func MergeMenuIDs(templateMenus []*Menu, explicitIDs []uint) []uint {
	set := make(map[uint]struct{})

	for _, menu := range templateMenus {
		if menu != nil {
			set[menu.ID] = struct{}{}
		}
	}

	for _, id := range explicitIDs {
		if id > 0 {
			set[id] = struct{}{}
		}
	}

	ids := make([]uint, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}

	return ids
}
