package rbac

import "time"

// Role 角色实体（RBAC 上下文的聚合根）。
//
// 字段与 json tag 与原 models/mysqlEntity.Role 对外序列化部分保持一致（id/name/desc/menus），
// 保证 GET /rbac/roles 等接口响应 JSON 1:1。
// 原实体的反向关联 Users 属于 identity 上下文且从未被任何接口序列化输出
// （删除角色时只用于「是否仍有用户占用」判断），故领域实体不予保留，改由仓储提供计数方法。
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

// DedupeMenuIDs 过滤掉集合 exclude 中的菜单 ID 并去重，保持入参顺序。
//
// 对应原 assign_menu.go / remove_menu.go 中「排除继承菜单 + 去重」的公共前置逻辑。
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
// 语义与原 create_role.go 一致：模板角色的菜单全量纳入，显式菜单仅纳入正数 ID。
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
