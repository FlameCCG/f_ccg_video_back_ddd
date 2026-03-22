package rbac

import (
	"strings"
	"time"
)

// Role 角色实体，是 RBAC 上下文的聚合根。
//
// 状态不对外暴露，HTTP 响应使用应用层或 handler 的查询 DTO，字段为 id/name/desc/menus。
// 领域实体不持有 Users 反向关联；删除角色时通过仓储计数判断是否有用户占用。
type Role struct {
	id        uint
	createdAt time.Time
	updatedAt time.Time
	name      string
	desc      string
	menus     []*Menu
}

// RoleSnapshot 是角色的持久化与历史重建快照，字段公开。
type RoleSnapshot struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Desc      string
	Menus     []MenuSnapshot
}

// RoleMenu 角色-菜单关联（含「是否由继承而来」标记）。
type RoleMenu struct {
	RoleID      uint
	MenuID      uint
	IsInherited bool
}

// NewRole 构造角色实体。名称唯一性由用例在持久化前确认，这里不新增限制。
func NewRole(name, desc string) *Role {
	return &Role{name: name, desc: desc}
}

// RestoreRole 从快照恢复历史角色，不重新执行创建校验。
func RestoreRole(snapshot RoleSnapshot) *Role {
	role := &Role{
		id:        snapshot.ID,
		createdAt: snapshot.CreatedAt,
		updatedAt: snapshot.UpdatedAt,
		name:      snapshot.Name,
		desc:      snapshot.Desc,
	}
	if snapshot.Menus != nil {
		role.menus = make([]*Menu, len(snapshot.Menus))
		for i := range snapshot.Menus {
			role.menus[i] = RestoreMenu(snapshot.Menus[i])
		}
	}

	return role
}

// Snapshot 复制当前角色。修改返回值不会影响实体。
func (r *Role) Snapshot() RoleSnapshot {
	if r == nil {
		return RoleSnapshot{}
	}

	snapshot := RoleSnapshot{
		ID:        r.id,
		CreatedAt: r.createdAt,
		UpdatedAt: r.updatedAt,
		Name:      r.name,
		Desc:      r.desc,
	}
	if r.menus != nil {
		snapshot.Menus = make([]MenuSnapshot, len(r.menus))
		for i, menu := range r.menus {
			if menu != nil {
				snapshot.Menus[i] = menu.Snapshot()
			}
		}
	}

	return snapshot
}

func (r *Role) ID() uint {
	if r == nil {
		return 0
	}

	return r.id
}
func (r *Role) CreatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.createdAt
}
func (r *Role) UpdatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.updatedAt
}
func (r *Role) Name() string {
	if r == nil {
		return ""
	}

	return r.name
}
func (r *Role) Desc() string {
	if r == nil {
		return ""
	}

	return r.desc
}

// Menus 返回菜单副本。调用方修改副本或其中的父级指针不会影响角色。
func (r *Role) Menus() []*Menu {
	if r == nil || r.menus == nil {
		return nil
	}

	cloned := make([]*Menu, len(r.menus))
	for i, menu := range r.menus {
		cloned[i] = menu.clone()
	}

	return cloned
}

// CheckedName 返回去掉空白后的角色名。继承树过滤拒绝空名。
func (r *Role) CheckedName() (string, error) {
	if r == nil {
		return "", ErrRoleNotFound
	}

	name := strings.TrimSpace(r.name)
	if name == "" {
		return "", ErrRoleNameBlank
	}

	return name, nil
}

// EnsureNameAvailable 同名角色已存在时拒绝创建或改名。
func (r *Role) EnsureNameAvailable(count int64) error {
	if r == nil {
		return ErrRoleNotFound
	}
	if count > 0 {
		return ErrRoleNameExists
	}

	return nil
}

// EnsureCanInherit 拒绝缺失角色和自己继承自己。循环与重复继承由继承服务另判。
func (r *Role) EnsureCanInherit(parent *Role) error {
	if r == nil {
		return ErrRoleNotFound
	}
	if parent == nil {
		return ErrParentRoleNotFound
	}
	if r.id == parent.id {
		return ErrParentRoleIsSelf
	}

	return nil
}

// MergedMenuIDs 把模板角色菜单与显式指定的菜单 ID 合并为去重集合。
//
// 语义模板角色的菜单全量纳入，显式菜单仅纳入正数 ID。
func (Role) MergedMenuIDs(templateMenus []*Menu, explicitIDs []uint) []uint {
	set := make(map[uint]struct{})
	for _, menu := range templateMenus {
		if menu != nil {
			set[menu.ID()] = struct{}{}
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

		items = append(items, RoleMenu{RoleID: roleID, MenuID: menu.ID(), IsInherited: false})
	}

	return items
}
