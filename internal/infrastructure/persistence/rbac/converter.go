// Package rbac 是 RBAC 上下文的持久化实现：GORM 仓储、Casbin 适配器与 PO↔domain 转换。
package rbac

import (
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/datatypes"
)

// toDomainRole 把角色 PO 转为领域实体（含已预加载的菜单）。
func toDomainRole(row *po.Role) *domain.Role {
	if row == nil {
		return nil
	}

	return &domain.Role{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Name:      row.Name,
		Desc:      row.Desc,
		Menus:     toDomainMenus(row.Menus),
	}
}

// toDomainRoles 批量转换角色 PO。
func toDomainRoles(rows []po.Role) []*domain.Role {
	roles := make([]*domain.Role, 0, len(rows))
	for i := range rows {
		roles = append(roles, toDomainRole(&rows[i]))
	}

	return roles
}

// toDomainRolePtrs 批量转换角色 PO 指针切片（预加载关联的产物）。
//
// 入参为 nil 时返回 nil，保持与 GORM 预加载结果一致的「空/未加载」语义，
// 避免把未加载的关联序列化成 []。
func toDomainRolePtrs(rows []*po.Role) []*domain.Role {
	if rows == nil {
		return nil
	}

	roles := make([]*domain.Role, 0, len(rows))

	for _, row := range rows {
		if row == nil {
			continue
		}

		roles = append(roles, toDomainRole(row))
	}

	return roles
}

// toPORole 把角色领域实体转为 PO（不含关联）。
func toPORole(role *domain.Role) po.Role {
	if role == nil {
		return po.Role{}
	}

	return po.Role{
		Model: po.Model{
			ID:        role.ID,
			CreatedAt: role.CreatedAt,
			UpdatedAt: role.UpdatedAt,
		},
		Name: role.Name,
		Desc: role.Desc,
	}
}

// toDomainMenu 把菜单 PO 转为领域实体。
//
// Children 不参与持久化（PO 上为 gorm:"-"），由领域层的 BuildMenuTree 组装。
func toDomainMenu(row *po.Menu) *domain.Menu {
	if row == nil {
		return nil
	}

	return &domain.Menu{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Title:     row.Title,
		TitleEn:   row.TitleEn,
		TitleJa:   row.TitleJa,
		Icon:      row.Icon,
		Path:      row.Path,
		Name:      row.Name,
		Component: row.Component,
		KeepAlive: row.KeepAlive,
		ParentID:  row.ParentID,
		SortOrder: row.SortOrder,
	}
}

// toDomainMenus 批量转换菜单 PO 指针切片。
func toDomainMenus(rows []*po.Menu) []*domain.Menu {
	if rows == nil {
		return nil
	}

	menus := make([]*domain.Menu, 0, len(rows))

	for _, row := range rows {
		if row == nil {
			continue
		}

		menus = append(menus, toDomainMenu(row))
	}

	return menus
}

// toDomainMenuValues 批量转换菜单 PO 值切片。
func toDomainMenuValues(rows []po.Menu) []*domain.Menu {
	menus := make([]*domain.Menu, 0, len(rows))
	for i := range rows {
		menus = append(menus, toDomainMenu(&rows[i]))
	}

	return menus
}

// toPOMenu 把菜单领域实体转为 PO。
func toPOMenu(menu *domain.Menu) po.Menu {
	if menu == nil {
		return po.Menu{}
	}

	return po.Menu{
		Model: po.Model{
			ID:        menu.ID,
			CreatedAt: menu.CreatedAt,
			UpdatedAt: menu.UpdatedAt,
		},
		Title:     menu.Title,
		TitleEn:   menu.TitleEn,
		TitleJa:   menu.TitleJa,
		Icon:      menu.Icon,
		Path:      menu.Path,
		Name:      menu.Name,
		Component: menu.Component,
		KeepAlive: menu.KeepAlive,
		ParentID:  menu.ParentID,
		SortOrder: menu.SortOrder,
	}
}

// toPOMenus 批量转换菜单领域实体。
func toPOMenus(menus []*domain.Menu) []po.Menu {
	rows := make([]po.Menu, 0, len(menus))

	for _, menu := range menus {
		if menu == nil {
			continue
		}

		rows = append(rows, toPOMenu(menu))
	}

	return rows
}

// toPORoleMenus 批量转换角色-菜单关联。
func toPORoleMenus(items []domain.RoleMenu) []po.RoleMenu {
	rows := make([]po.RoleMenu, 0, len(items))
	for _, item := range items {
		rows = append(rows, po.RoleMenu{
			RoleID:      item.RoleID,
			MenuID:      item.MenuID,
			IsInherited: item.IsInherited,
		})
	}

	return rows
}

// toDomainApiResource 把接口资源 PO 转为领域实体。
func toDomainApiResource(row *po.ApiResource) *domain.ApiResource {
	if row == nil {
		return nil
	}

	return &domain.ApiResource{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		Path:      row.Path,
		Method:    row.Method,
		Summary:   row.Summary,
		Tags:      []byte(row.Tags),
	}
}

// toDomainApiResources 批量转换接口资源 PO。
func toDomainApiResources(rows []po.ApiResource) []*domain.ApiResource {
	items := make([]*domain.ApiResource, 0, len(rows))
	for i := range rows {
		items = append(items, toDomainApiResource(&rows[i]))
	}

	return items
}

// toPOApiResource 把接口资源领域实体转为 PO。
func toPOApiResource(item *domain.ApiResource) po.ApiResource {
	if item == nil {
		return po.ApiResource{}
	}

	return po.ApiResource{
		Model: po.Model{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		Path:    item.Path,
		Method:  item.Method,
		Summary: item.Summary,
		Tags:    datatypes.JSON(item.Tags),
	}
}
