package router

import (
	rbachandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/rbac"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gin-gonic/gin"
)

// registerRbacRoutes 装配 /rbac 下的全部路由（method+path 与原 router/rbac_router.go 完全一致）。
func registerRbacRoutes(g *gin.RouterGroup, mw *middlewares.Middlewares, h *rbachandler.Handler) {
	rbacGroup := g.Group("/rbac")

	authGroup := rbacGroup.Group("")
	authGroup.Use(middlewares.RequestTimeout(ctxutil.RequestSlowTimeout), mw.AuthAll())

	// 角色管理
	{
		authGroup.POST("/role", middlewares.JSONBindParams[rbachandler.CreateRoleReq], h.CreateRoleView)                   // 创建角色
		authGroup.PUT("/role", middlewares.JSONBindParams[rbachandler.UpdateRoleReq], h.UpdateRoleView)                    // 更新角色
		authGroup.GET("/roles", h.GetRoleListView)                                                                         // 获取角色列表
		authGroup.DELETE("/role", h.DeleteRoleView)                                                                        // 删除角色 ?id=
		authGroup.POST("/role/assign", middlewares.JSONBindParams[rbachandler.AssignRoleReq], h.AssignRoleView)            // 分配用户角色
		authGroup.DELETE("/role/remove", middlewares.JSONBindParams[rbachandler.AssignRoleReq], h.RemoveRoleView)          // 移除用户角色
		authGroup.PUT("/user/roles", middlewares.JSONBindParams[rbachandler.UpdateUserRolesReq], h.UpdateUserRolesView)    // 更新用户角色
		authGroup.GET("/user/roles", h.GetUserRolesView)                                                                   // 获取用户角色 ?userId=
		authGroup.GET("/user/permissions", h.UserPermissionsView)                                                          // 获取用户权限
		authGroup.GET("/resources", h.ResourceListView)                                                                    // 资源列表
		authGroup.POST("/role/inherit", middlewares.JSONBindParams[rbachandler.RoleInheritReq], h.AssignRoleInheritView)   // 角色继承
		authGroup.DELETE("/role/inherit", middlewares.JSONBindParams[rbachandler.RoleInheritReq], h.RemoveRoleInheritView) //
		authGroup.GET("/role/inherits", h.RoleInheritListView)                                                             // 角色继承列表
		authGroup.GET("/role/inherit/tree", h.RoleInheritTreeView)                                                         // 角色继承树（可选 ?roleId= 过滤）
		authGroup.GET("/role/menus", h.RoleMenusView)                                                                      // 根据角色ID获取菜单列表
	}

	// 菜单管理
	{
		authGroup.POST("/menu", middlewares.JSONBindParams[rbachandler.CreateMenuReq], h.CreateMenuView)              // 创建菜单
		authGroup.PUT("/menu", middlewares.JSONBindParams[rbachandler.UpdateMenuReq], h.UpdateMenuView)               // 更新菜单
		authGroup.PUT("/menu/move", middlewares.JSONBindParams[rbachandler.MoveMenuReq], h.MoveMenuView)              // 拖拽排序菜单
		authGroup.POST("/menu/generate", middlewares.JSONBindParams[rbachandler.GenerateMenuReq], h.GenerateMenuView) // AI 生成菜单字段
		authGroup.DELETE("/menu", h.DeleteMenuView)                                                                   // 删除菜单 ?id=
		authGroup.POST("/menu/assign", middlewares.JSONBindParams[rbachandler.AssignMenuReq], h.AssignMenuView)       // 分配菜单
		authGroup.GET("/menus", h.GetMenuListView)                                                                    // 获取所有菜单
		authGroup.GET("/user/menus", h.GetUserMenusView)                                                              // 获取当前用户菜单
		authGroup.DELETE("/menu/remove", middlewares.JSONBindParams[rbachandler.AssignMenuReq], h.RemoveRoleMenuView) // 移除角色菜单
	}

	// 权限管理
	{
		authGroup.POST("/permission", middlewares.JSONBindParams[rbachandler.AssignPermissionReq], h.AssignPermissionView)                // 分配权限
		authGroup.DELETE("/permission", middlewares.JSONBindParams[rbachandler.AssignPermissionReq], h.RemovePermissionView)              // 移除权限
		authGroup.PUT("/role/permissions", middlewares.JSONBindParams[rbachandler.UpdateRolePermissionsReq], h.UpdateRolePermissionsView) // 替换角色权限
		authGroup.GET("/role/permissions", h.GetRolePermissionsView)                                                                      // 获取角色权限
		authGroup.GET("/permission/wildcards", h.WildcardListView)                                                                        // 系统通配符权限列表
	}
}
