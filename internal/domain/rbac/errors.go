package rbac

import (
	"errors"
	"fmt"
)

// 领域层错误：文案与原 api/rbac_api、service/rbac_service 中直接下发给前端的字符串逐字一致，
// 由 handler 以 res.FailWithMsg(err.Error(), c) 渲染，保证接口错误文案与状态码 1:1。
var (
	// ErrCreateFailed 创建失败.
	ErrCreateFailed = errors.New("创建失败")
	// ErrUpdateFailed 更新失败.
	ErrUpdateFailed = errors.New("更新失败")
	// ErrDeleteFailed 删除失败.
	ErrDeleteFailed = errors.New("删除失败")
	// ErrGetFailed 获取失败.
	ErrGetFailed = errors.New("获取失败")
	// ErrQueryFailed 查询失败.
	ErrQueryFailed = errors.New("查询失败")
	// ErrAddFailed 添加失败.
	ErrAddFailed = errors.New("添加失败")
	// ErrSaveFailed 保存失败.
	ErrSaveFailed = errors.New("保存失败")
	// ErrAssignFailed 分配失败.
	ErrAssignFailed = errors.New("分配失败")
	// ErrRemoveFailed 移除失败.
	ErrRemoveFailed = errors.New("移除失败")
	// ErrNoUpdatableFields 没有需要更新的字段.
	ErrNoUpdatableFields = errors.New("没有需要更新的字段")
)

// 角色相关错误。
var (
	// ErrRoleNotFound 角色不存在.
	ErrRoleNotFound = errors.New("角色不存在")
	// ErrRoleNameExists 角色名已存在.
	ErrRoleNameExists = errors.New("角色名已存在")
	// ErrRoleNameEmpty 角色名称不能为空.
	ErrRoleNameEmpty = errors.New("角色名称不能为空")
	// ErrRoleNameBlank 角色名称为空（继承树按 roleId 过滤时角色名异常）.
	ErrRoleNameBlank = errors.New("角色名称为空")
	// ErrRoleHasUsers 该角色下还有用户，无法删除.
	ErrRoleHasUsers = errors.New("该角色下还有用户，无法删除")
	// ErrRolePolicyCleanupFailed 角色已删除，但权限策略清理失败.
	ErrRolePolicyCleanupFailed = errors.New("角色已删除，但权限策略清理失败")
	// ErrRolePolicySaveFailed 角色已删除，但权限策略保存失败.
	ErrRolePolicySaveFailed = errors.New("角色已删除，但权限策略保存失败")
	// ErrTemplateRoleNotFound 模板角色不存在.
	ErrTemplateRoleNotFound = errors.New("模板角色不存在")
	// ErrTemplatePermissionsRead 读取模板权限失败.
	ErrTemplatePermissionsRead = errors.New("读取模板权限失败")
	// ErrBindMenuFailed 绑定菜单失败.
	ErrBindMenuFailed = errors.New("绑定菜单失败")
	// ErrQueryRoleFailed 查询角色失败.
	ErrQueryRoleFailed = errors.New("查询角色失败")
	// ErrSomeRolesNotFound 某些角色不存在.
	ErrSomeRolesNotFound = errors.New("某些角色不存在")
	// ErrInvalidCasbinPermission Casbin 策略行残缺（重命名角色时无法迁移）.
	ErrInvalidCasbinPermission = errors.New("invalid casbin permission")
)

// 用户相关错误（rbac 侧的用户校验文案）。
var (
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrQueryUserFailed 查询用户失败.
	ErrQueryUserFailed = errors.New("查询用户失败")
)

// 菜单相关错误。
var (
	// ErrMenuNotFound 菜单不存在.
	ErrMenuNotFound = errors.New("菜单不存在")
	// ErrMenuExists 菜单已存在.
	ErrMenuExists = errors.New("菜单已存在")
	// ErrPartialMenusNotFound 部分菜单不存在.
	ErrPartialMenusNotFound = errors.New("部分菜单不存在")
	// ErrSomeMenusNotFound 某些菜单不存在.
	ErrSomeMenusNotFound = errors.New("某些菜单不存在")
	// ErrQueryMenuFailed 查询菜单失败.
	ErrQueryMenuFailed = errors.New("查询菜单失败")
	// ErrMenuTitleEmpty 菜单标题不能为空.
	ErrMenuTitleEmpty = errors.New("菜单标题不能为空")
	// ErrMenuTitleEnEmpty 菜单英文标题不能为空.
	ErrMenuTitleEnEmpty = errors.New("菜单英文标题不能为空")
	// ErrMenuTitleJaEmpty 菜单日文标题不能为空.
	ErrMenuTitleJaEmpty = errors.New("菜单日文标题不能为空")
	// ErrMenuPathEmpty 路由路径不能为空.
	ErrMenuPathEmpty = errors.New("路由路径不能为空")
	// ErrMenuNameEmpty 路由名称不能为空.
	ErrMenuNameEmpty = errors.New("路由名称不能为空")
	// ErrMenuComponentEmpty 组件标识不能为空.
	ErrMenuComponentEmpty = errors.New("组件标识不能为空")
	// ErrParentMenuNotFound 父菜单不存在.
	ErrParentMenuNotFound = errors.New("父菜单不存在")
	// ErrParentMenuIsSelf 父菜单不能是自己.
	ErrParentMenuIsSelf = errors.New("父菜单不能是自己")
	// ErrDeleteChildMenusFirst 请先删除子菜单.
	ErrDeleteChildMenusFirst = errors.New("请先删除子菜单")
)

// 权限相关错误。
var (
	// ErrPermissionsRequired permissions 不能为空.
	ErrPermissionsRequired = errors.New("permissions 不能为空")
	// ErrPermissionParamIncomplete 权限参数不完整.
	ErrPermissionParamIncomplete = errors.New("权限参数不完整")
	// ErrPermissionAlreadyOwned 角色（含继承）已具备目标权限.
	ErrPermissionAlreadyOwned = errors.New("您已有此权限")
	// ErrPermissionSyncFailed 权限同步失败.
	ErrPermissionSyncFailed = errors.New("权限同步失败")
	// ErrPermissionSaveFailed 权限保存失败.
	ErrPermissionSaveFailed = errors.New("权限保存失败")
	// ErrAddPermissionFailed 添加权限失败.
	ErrAddPermissionFailed = errors.New("添加权限失败")
	// ErrSavePermissionFailed 保存权限失败.
	ErrSavePermissionFailed = errors.New("保存权限失败")
	// ErrQueryPermissionFailed 查询权限失败.
	ErrQueryPermissionFailed = errors.New("查询权限失败")
	// ErrGetPermissionFailed 获取权限失败.
	ErrGetPermissionFailed = errors.New("获取权限失败")
	// ErrGetResourceFailed 获取资源失败.
	ErrGetResourceFailed = errors.New("获取资源失败")
	// ErrGetWildcardFailed 获取通配符权限失败.
	ErrGetWildcardFailed = errors.New("获取通配符权限失败")
)

// 角色继承相关错误。
var (
	// ErrParentRoleNotFound 父角色不存在.
	ErrParentRoleNotFound = errors.New("父角色不存在")
	// ErrParentRoleIsSelf 父角色不能是自己.
	ErrParentRoleIsSelf = errors.New("父角色不能是自己")
	// ErrInheritFailed 继承失败.
	ErrInheritFailed = errors.New("继承失败")
	// ErrInheritCycle 不能继承该角色，会形成循环继承.
	ErrInheritCycle = errors.New("不能继承该角色，会形成循环继承")
	// ErrNotDirectInherit 未直接继承该角色，无法解除.
	ErrNotDirectInherit = errors.New("未直接继承该角色，无法解除")
	// ErrBreakInheritanceFailed 解除继承失败.
	ErrBreakInheritanceFailed = errors.New("解除继承失败")
	// ErrEnforcerNotInitialized 权限未初始化（Casbin 端口缺失）.
	ErrEnforcerNotInitialized = errors.New("权限未初始化")
)

// 初始化相关错误（原 service/rbac_service/init_service.go，文案为英文，保持原样）。
var (
	// ErrCasbinNotInitialized casbin 未初始化.
	ErrCasbinNotInitialized = errors.New("casbin not initialized")
	// ErrRoleInitFailed 默认角色初始化失败.
	ErrRoleInitFailed = errors.New("role init failed")
	// ErrNoRouteFound 未发现任何路由（资源同步）.
	ErrNoRouteFound = errors.New("未发现任何路由")
	// ErrOpenAPINotFound 未找到 OpenAPI 文件.
	ErrOpenAPINotFound = errors.New("未找到 OpenAPI 文件")
	// ErrOpenAPIRead 读取 OpenAPI 失败.
	ErrOpenAPIRead = errors.New("读取 OpenAPI 失败")
	// ErrOpenAPIParse 解析 OpenAPI 失败.
	ErrOpenAPIParse = errors.New("解析 OpenAPI 失败")
)

// DuplicateRoleAssignedError 部分角色已分配（携带首个重复角色名，文案与原实现一致）。
func DuplicateRoleAssignedError(roleName string) error {
	return fmt.Errorf("部分角色已分配: %s", roleName)
}

// AlreadyInheritedError 已直接/间接继承目标角色时的业务错误。
func AlreadyInheritedError(targetRole string, kind InheritKind, topParent string) error {
	return errors.New(FormatAlreadyInheritedMsg(targetRole, kind, topParent))
}

// IndirectUnlinkError 尝试解除间接继承时的业务错误。
func IndirectUnlinkError(targetRole, topParent string) error {
	return errors.New(FormatIndirectUnlinkMsg(targetRole, topParent))
}
