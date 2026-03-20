package rbac

// Enforcer Casbin 策略端口，实现在 infrastructure/persistence/rbac/enforcer.go。
//
// 应用层不直接依赖 casbin，SavePolicy 会重写完整策略集。
type Enforcer interface {
	// GetPermissionsForUser 读取主体的直接 p 策略。
	GetPermissionsForUser(user string) ([][]string, error)
	// GetImplicitPermissionsForUser 读取主体的有效 p 策略（含继承）。
	GetImplicitPermissionsForUser(user string) ([][]string, error)
	// GetPolicy 读取全部 p 策略。
	GetPolicy() ([][]string, error)
	// GetNamedGroupingPolicy 读取指定名称的分组策略（角色继承关系）。
	GetNamedGroupingPolicy(ptype string) ([][]string, error)
	// GetRolesForUser 读取主体的直接父角色。
	GetRolesForUser(name string) ([]string, error)
	// GetImplicitRolesForUser 读取主体的全部隐式父角色（含间接，不含自身）。
	GetImplicitRolesForUser(name string) ([]string, error)
	// GetUsersForRole 读取直接继承该角色的主体（用户或子角色）。
	GetUsersForRole(name string) ([]string, error)
	// AddPolicy 新增一条 p 策略。
	AddPolicy(sub, obj, act string) (bool, error)
	// RemovePolicy 移除一条 p 策略。
	RemovePolicy(sub, obj, act string) (bool, error)
	// RemoveFilteredPolicy 按字段过滤批量移除 p 策略。
	RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error)
	// AddRoleForUser 建立「user 继承 role」的分组策略。
	AddRoleForUser(user, role string) (bool, error)
	// DeleteRoleForUser 解除「user 继承 role」的分组策略。
	DeleteRoleForUser(user, role string) (bool, error)
	// DeleteRole 删除角色及其全部策略与分组关系。
	DeleteRole(role string) (bool, error)
	// SavePolicy 将内存策略整体落库。
	SavePolicy() error
}
