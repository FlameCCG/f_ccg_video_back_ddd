package rbac

import "context"

// RoleRepository 角色聚合仓储（接口所有权归领域层）。
type RoleRepository interface {
	// FindByID 按主键读取角色；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, roleID uint) (*Role, error)
	// FindBriefByID 只读 id/name/desc；不存在时返回 (nil, nil)。
	FindBriefByID(ctx context.Context, roleID uint) (*Role, error)
	// FindIDByID 只读主键，用于「角色是否存在」校验；不存在时返回 (nil, nil)。
	FindIDByID(ctx context.Context, roleID uint) (*Role, error)
	// FindByName 按角色名读取；不存在时返回 (nil, nil)。
	FindByName(ctx context.Context, name string) (*Role, error)
	// FindWithMenusByID 读取角色并预加载其菜单；不存在时返回 (nil, nil)。
	FindWithMenusByID(ctx context.Context, roleID uint) (*Role, error)
	// FindByIDs 批量读取角色（顺序由数据库决定）。
	FindByIDs(ctx context.Context, roleIDs []uint) ([]*Role, error)
	// FindBriefByNames 按角色名批量读取 id/name。
	FindBriefByNames(ctx context.Context, names []string) ([]*Role, error)
	// CountByName 统计同名角色数量。
	CountByName(ctx context.Context, name string) (int64, error)
	// ListWithMenus 读取全部角色并预加载菜单（角色列表接口）。
	ListWithMenus(ctx context.Context) ([]*Role, error)
	// ListBrief 读取全部角色的 id/name/desc（继承树建树用）。
	ListBrief(ctx context.Context) ([]*Role, error)
	// UserCount 统计仍绑定该角色的用户数。
	UserCount(ctx context.Context, roleID uint) (int64, error)
	// Create 创建角色并回填自增 ID。
	Create(ctx context.Context, role *Role) error
	// UpdateNameAndDesc 更新角色名与描述；两者均为 nil 时不产生 UPDATE。
	UpdateNameAndDesc(ctx context.Context, roleID uint, name, desc *string) error
	// UpdateName 覆盖写角色名（角色名同步 Casbin 失败后的回滚路径）。
	UpdateName(ctx context.Context, roleID uint, name string) error
	// DeleteByID 删除角色（触发清理角色-菜单、角色-用户关联的持久化钩子）。
	DeleteByID(ctx context.Context, roleID uint) error
	// DeleteBindings removes role links and resets stale current-role selections.
	DeleteBindings(ctx context.Context, roleID uint) error
	// AppendMenus 向角色追加直连菜单关联。
	AppendMenus(ctx context.Context, role *Role, menus []*Menu) error
}

// MenuRepository 菜单仓储。
type MenuRepository interface {
	// FindByID 按主键读取菜单；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, menuID uint) (*Menu, error)
	// FindByIDForUpdate 按主键读取并锁定菜单；不存在时返回 (nil, nil)。
	FindByIDForUpdate(ctx context.Context, menuID uint) (*Menu, error)
	// FindByIDsForUpdate 批量读取并锁定菜单。
	FindByIDsForUpdate(ctx context.Context, menuIDs []uint) ([]*Menu, error)
	// FindByTitle 按标题读取菜单；不存在时返回 (nil, nil)。
	FindByTitle(ctx context.Context, title string) (*Menu, error)
	// ExistsByID 判断菜单是否存在。
	ExistsByID(ctx context.Context, menuID uint) (bool, error)
	// FindByIDs 批量读取菜单。
	FindByIDs(ctx context.Context, menuIDs []uint) ([]*Menu, error)
	// ListOrdered 按 sort_order 升序读取全部菜单。
	ListOrdered(ctx context.Context) ([]*Menu, error)
	// ListForGeneration 按 parent_id、sort_order、id 读取完整菜单提示词数据。
	ListForGeneration(ctx context.Context) ([]*Menu, error)
	// ListSiblingIDsForUpdate 按 sort_order、id 读取并锁定同层菜单 ID。
	ListSiblingIDsForUpdate(ctx context.Context, parentID *uint) ([]uint, error)
	// ListByRole 读取角色的全部菜单（不区分是否继承），按 sort_order 升序去重。
	ListByRole(ctx context.Context, roleID uint) ([]*Menu, error)
	// ListByRoleInherited 读取角色的直连或继承菜单，按 sort_order 升序去重。
	ListByRoleInherited(ctx context.Context, roleID uint, inherited bool) ([]*Menu, error)
	// CountChildren 统计子菜单数量。
	CountChildren(ctx context.Context, parentID uint) (int64, error)
	// CountByTitleExcluding 统计同标题且排除指定主键的菜单数量。
	CountByTitleExcluding(ctx context.Context, title string, excludeID uint) (int64, error)
	// Create 创建菜单并回填自增 ID。
	Create(ctx context.Context, menu *Menu) error
	// UpdateFields 按需更新菜单字段；update 为空时不产生 UPDATE。
	UpdateFields(ctx context.Context, menuID uint, update MenuFieldUpdate) error
	// UpdateSortOrder 更新单个菜单的同层顺序。
	UpdateSortOrder(ctx context.Context, menuID, sortOrder uint) error
	// DeleteByID 删除菜单。
	DeleteByID(ctx context.Context, menuID uint) error
}

// RoleMenuRepository 角色-菜单关联仓储。
type RoleMenuRepository interface {
	// InheritedMenuIDs 读取角色的继承菜单 ID 集合。
	InheritedMenuIDs(ctx context.Context, roleID uint) (map[uint]struct{}, error)
	// MenuIDsByRole 读取角色的全部菜单 ID（不去重，不区分是否继承）。
	MenuIDsByRole(ctx context.Context, roleID uint) ([]uint, error)
	// DistinctMenuIDsByRole 读取角色的菜单 ID（去重）。
	DistinctMenuIDsByRole(ctx context.Context, roleID uint) ([]uint, error)
	// DistinctMenuIDsByRoles 读取多个角色的菜单 ID 并集（去重）。
	DistinctMenuIDsByRoles(ctx context.Context, roleIDs []uint) ([]uint, error)
	// CreateIgnoreConflict 批量写入关联，冲突行忽略。
	CreateIgnoreConflict(ctx context.Context, items []RoleMenu) error
	// EnsureRoleMenu 幂等确保存在一条 (roleID, menuID, is_inherited=false) 关联。
	EnsureRoleMenu(ctx context.Context, roleID, menuID uint) error
	// GrantDirect 幂等授予直连菜单；已有继承关联时改为直连。
	GrantDirect(ctx context.Context, roleID, menuID uint) error
	// DeleteDirectByRole 删除角色的全部直连关联。
	DeleteDirectByRole(ctx context.Context, roleID uint) error
	// DeleteDirectByRoleExcept 删除角色除 keepMenuIDs 之外的直连关联。
	DeleteDirectByRoleExcept(ctx context.Context, roleID uint, keepMenuIDs []uint) error
	// DeleteDirectByRoleAndMenus 删除角色在 menuIDs 内的直连关联。
	DeleteDirectByRoleAndMenus(ctx context.Context, roleID uint, menuIDs []uint) error
	// DeleteInheritedByRole 删除角色的全部继承关联。
	DeleteInheritedByRole(ctx context.Context, roleID uint) error
	// DeleteInheritedByRoleExcept 删除角色除 keepMenuIDs 之外的继承关联。
	DeleteInheritedByRoleExcept(ctx context.Context, roleID uint, keepMenuIDs []uint) error
	// DeleteByMenu 删除某个菜单的全部角色关联。
	DeleteByMenu(ctx context.Context, menuID uint) error
}

// UserRoleRepository 用户-角色绑定仓储。
//
// 「用户拥有哪些角色」属于 rbac 上下文的职责，故仓储接口定义在本领域；
// 其实现需要读 identity 的 users 表（存在性与 current_role_id），
// 按 docs/architecture.md 第 4 节 的读侧投影约定直读，不引入跨上下文调用链。
type UserRoleRepository interface {
	// UserExists 判断用户是否存在。
	UserExists(ctx context.Context, userID uint) (bool, error)
	// CurrentRoleID 读取用户当前角色 ID；用户不存在时 found 为 false。
	CurrentRoleID(ctx context.Context, userID uint) (roleID uint, found bool, err error)
	// RolesByUser 读取用户已绑定的角色；用户不存在时返回 ErrUserNotFound。
	RolesByUser(ctx context.Context, userID uint) ([]*Role, error)
	// AppendRoles 为用户追加角色绑定。
	AppendRoles(ctx context.Context, userID uint, roles []*Role) error
	// RemoveRoles 移除用户的角色绑定。
	RemoveRoles(ctx context.Context, userID uint, roles []*Role) error
	// ReplaceRoles 用给定集合整体替换用户的角色绑定。
	ReplaceRoles(ctx context.Context, userID uint, roles []*Role) error
}

// APIResourceRepository 接口资源仓储。
type APIResourceRepository interface {
	// ListOrdered 按 path、method 升序读取全部资源。
	ListOrdered(ctx context.Context) ([]*APIResource, error)
	// ListPathMethod 只读 path、method（权限覆盖计算用）。
	ListPathMethod(ctx context.Context) ([]*APIResource, error)
	// ListAll 读取全部资源（同步比对用）。
	ListAll(ctx context.Context) ([]*APIResource, error)
	// Create 创建资源。
	Create(ctx context.Context, resource *APIResource) error
	// UpdateSummaryAndTags 更新资源说明与标签；两者均为 nil 时不产生 UPDATE。
	UpdateSummaryAndTags(ctx context.Context, resourceID uint, summary *string, tags *[]byte) error
	// DeleteByID 删除资源。
	DeleteByID(ctx context.Context, resourceID uint) error
}
