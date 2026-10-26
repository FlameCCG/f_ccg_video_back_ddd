package rbac

import (
	app "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/util"
)

// CasbinEnforcer 把 Casbin SyncedEnforcer 适配为应用层的 Enforcer 端口。
//
// 应用层与领域层因此不 import casbin；方法体逐一委派，不改变任何策略语义
// （尤其 SavePolicy 依旧是「内存策略整体覆盖落库」）。
type CasbinEnforcer struct {
	enforcer *casbin.SyncedEnforcer
	locked   bool
}

// 编译期断言：满足应用层声明的端口。
var _ app.Enforcer = (*CasbinEnforcer)(nil)

// NewCasbinEnforcer 构造 Casbin 策略适配器。
func NewCasbinEnforcer(enforcer *casbin.SyncedEnforcer) *CasbinEnforcer {
	return &CasbinEnforcer{enforcer: enforcer}
}

// GetPermissionsForUser 读取主体的直接 p 策略。
func (e *CasbinEnforcer) GetPermissionsForUser(user string) ([][]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetPermissionsForUser(user)
	}

	return e.enforcer.GetPermissionsForUser(user)
}

// GetImplicitPermissionsForUser 读取主体的有效 p 策略（含继承）。
func (e *CasbinEnforcer) GetImplicitPermissionsForUser(user string) ([][]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetImplicitPermissionsForUser(user)
	}

	return e.enforcer.GetImplicitPermissionsForUser(user)
}

// GetPolicy 读取全部 p 策略。
func (e *CasbinEnforcer) GetPolicy() ([][]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetPolicy()
	}

	return e.enforcer.GetPolicy()
}

// GetNamedGroupingPolicy 读取指定名称的分组策略。
func (e *CasbinEnforcer) GetNamedGroupingPolicy(ptype string) ([][]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetNamedGroupingPolicy(ptype)
	}

	return e.enforcer.GetNamedGroupingPolicy(ptype)
}

// GetRolesForUser 读取主体的直接父角色。
func (e *CasbinEnforcer) GetRolesForUser(name string) ([]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetRolesForUser(name)
	}

	return e.enforcer.GetRolesForUser(name)
}

// GetImplicitRolesForUser 读取主体的全部隐式父角色。
func (e *CasbinEnforcer) GetImplicitRolesForUser(name string) ([]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetImplicitRolesForUser(name)
	}

	return e.enforcer.GetImplicitRolesForUser(name)
}

// GetUsersForRole 读取直接继承该角色的主体。
func (e *CasbinEnforcer) GetUsersForRole(name string) ([]string, error) {
	if e.locked {
		return e.enforcer.Enforcer.GetUsersForRole(name)
	}

	return e.enforcer.GetUsersForRole(name)
}

// AddPolicy 新增一条 p 策略。
func (e *CasbinEnforcer) AddPolicy(sub, obj, act string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.AddPolicy(sub, obj, act)
	}

	return e.enforcer.AddPolicy(sub, obj, act)
}

// RemovePolicy 移除一条 p 策略。
func (e *CasbinEnforcer) RemovePolicy(sub, obj, act string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.RemovePolicy(sub, obj, act)
	}

	return e.enforcer.RemovePolicy(sub, obj, act)
}

// RemoveFilteredPolicy 按字段过滤批量移除 p 策略。
func (e *CasbinEnforcer) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.RemoveFilteredPolicy(fieldIndex, fieldValues...)
	}

	return e.enforcer.RemoveFilteredPolicy(fieldIndex, fieldValues...)
}

// AddRoleForUser 建立「user 继承 role」的分组策略。
func (e *CasbinEnforcer) AddRoleForUser(user, role string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.AddRoleForUser(user, role)
	}

	return e.enforcer.AddRoleForUser(user, role)
}

// DeleteRoleForUser 解除「user 继承 role」的分组策略。
func (e *CasbinEnforcer) DeleteRoleForUser(user, role string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.DeleteRoleForUser(user, role)
	}

	return e.enforcer.DeleteRoleForUser(user, role)
}

// DeleteRole 删除角色及其全部策略与分组关系。
func (e *CasbinEnforcer) DeleteRole(role string) (bool, error) {
	if e.locked {
		return e.enforcer.Enforcer.DeleteRole(role)
	}

	return e.enforcer.DeleteRole(role)
}

// SavePolicy 将内存策略整体落库。
func (e *CasbinEnforcer) SavePolicy() error {
	if e.locked {
		return e.enforcer.Enforcer.SavePolicy()
	}

	return e.enforcer.SavePolicy()
}

// GlobPathMatcher 基于 Casbin 内置 globMatch 的路径匹配器。
//
// 权限覆盖判定必须与 conf/rbac_model.conf 中 matcher 使用的同一实现，
// 否则「已有权限是否覆盖待添加权限」的结论会与实际鉴权结果不一致。
type GlobPathMatcher struct{}

// 编译期断言：满足领域侧声明的匹配端口。
var _ domain.PathMatcher = GlobPathMatcher{}

// NewGlobPathMatcher 构造 glob 路径匹配器。
func NewGlobPathMatcher() GlobPathMatcher {
	return GlobPathMatcher{}
}

// Match 判断请求路径 request 是否匹配策略模式 pattern。
func (GlobPathMatcher) Match(request, pattern string) (bool, error) {
	return util.GlobMatch(request, pattern)
}

// NewPermissionEvaluator 用 glob 匹配器构造领域侧权限覆盖判定器。
func NewPermissionEvaluator() *domain.PermissionEvaluator {
	return domain.NewPermissionEvaluator(NewGlobPathMatcher())
}

// RestorePolicy restores the complete pre-operation model even when a mutation failed midway.
// AutoSave is disabled while rebuilding so a partial compensation cannot be persisted by a later save.
func (e *CasbinEnforcer) RestorePolicy(p, g [][]string) error {
	if !e.locked {
		e.enforcer.GetLock().Lock()
		defer e.enforcer.GetLock().Unlock()
	}

	engine := e.enforcer.Enforcer
	engine.EnableAutoSave(false)

	if !e.locked {
		defer engine.EnableAutoSave(true)
	}

	engine.ClearPolicy()

	if len(p) > 0 {
		if _, err := engine.AddPolicies(p); err != nil {
			return err
		}
	}

	if len(g) > 0 {
		if _, err := engine.AddGroupingPolicies(g); err != nil {
			return err
		}
	}

	return engine.SavePolicy()
}

// BeginPolicyMutation blocks authorization from observing an incomplete policy set.
// The returned adapter is private to the current operation and avoids reentrant locks.
func (e *CasbinEnforcer) BeginPolicyMutation() (app.Enforcer, func()) {
	e.enforcer.GetLock().Lock()
	e.enforcer.EnableAutoSave(false)

	return &CasbinEnforcer{enforcer: e.enforcer, locked: true}, func() { e.enforcer.EnableAutoSave(true); e.enforcer.GetLock().Unlock() }
}
