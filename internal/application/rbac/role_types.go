package rbac

import (
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
)

// CreateRoleParams 创建角色用例参数。
type CreateRoleParams struct {
	Name        string
	Desc        string
	MenuIDs     []uint
	Permissions []rbacvo.PermissionItem
	CopyRoleID  uint
}

// UpdateRoleParams 更新角色用例参数（指针为 nil 表示该字段不更新）。
type UpdateRoleParams struct {
	RoleID uint
	Name   *string
	Desc   *string
}

// RoleMenusResp 角色菜单详情响应（直连 + 继承，各自成树）。
type RoleMenusResp struct {
	DirectMenus  []*rbacentity.Menu `json:"directMenus"`
	InheritMenus []*rbacentity.Menu `json:"inheritMenus"`
}
