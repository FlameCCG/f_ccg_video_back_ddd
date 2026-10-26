package rbac_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/casbins"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/rbac"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fakeEnforcer 内存版 Casbin 策略实现，语义对齐 SyncedEnforcer 中被本上下文用到的方法。
//
// 单测不依赖真实 Casbin/MySQL（离线策略与仓库其余测试一致）。
type fakeEnforcer struct {
	policies [][]string // p 策略：sub, obj, act
	groups   [][]string // g 策略：child, parent
	saved    int
}

func (e *fakeEnforcer) GetPermissionsForUser(user string) ([][]string, error) {
	rows := make([][]string, 0)

	for _, p := range e.policies {
		if p[0] == user {
			rows = append(rows, []string{p[0], p[1], p[2]})
		}
	}

	return rows, nil
}

func (e *fakeEnforcer) GetImplicitPermissionsForUser(user string) ([][]string, error) {
	owners := []string{user}

	implicit, _ := e.GetImplicitRolesForUser(user)
	owners = append(owners, implicit...)

	rows := make([][]string, 0)

	for _, owner := range owners {
		own, _ := e.GetPermissionsForUser(owner)
		rows = append(rows, own...)
	}

	return rows, nil
}

func (e *fakeEnforcer) GetPolicy() ([][]string, error) {
	rows := make([][]string, 0, len(e.policies))
	for _, p := range e.policies {
		rows = append(rows, []string{p[0], p[1], p[2]})
	}

	return rows, nil
}

func (e *fakeEnforcer) GetNamedGroupingPolicy(string) ([][]string, error) {
	rows := make([][]string, 0, len(e.groups))
	for _, g := range e.groups {
		rows = append(rows, []string{g[0], g[1]})
	}

	return rows, nil
}

func (e *fakeEnforcer) GetRolesForUser(name string) ([]string, error) {
	parents := make([]string, 0)

	for _, g := range e.groups {
		if g[0] == name {
			parents = append(parents, g[1])
		}
	}

	return parents, nil
}

func (e *fakeEnforcer) GetImplicitRolesForUser(name string) ([]string, error) {
	visited := map[string]struct{}{name: {}}
	result := make([]string, 0)
	queue := []string{name}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		parents, _ := e.GetRolesForUser(current)
		for _, parent := range parents {
			if _, ok := visited[parent]; ok {
				continue
			}

			visited[parent] = struct{}{}
			result = append(result, parent)
			queue = append(queue, parent)
		}
	}

	return result, nil
}

func (e *fakeEnforcer) GetUsersForRole(name string) ([]string, error) {
	children := make([]string, 0)

	for _, g := range e.groups {
		if g[1] == name {
			children = append(children, g[0])
		}
	}

	return children, nil
}

func (e *fakeEnforcer) AddPolicy(sub, obj, act string) (bool, error) {
	for _, p := range e.policies {
		if p[0] == sub && p[1] == obj && p[2] == act {
			return false, nil
		}
	}

	e.policies = append(e.policies, []string{sub, obj, act})

	return true, nil
}

func (e *fakeEnforcer) RemovePolicy(sub, obj, act string) (bool, error) {
	kept := make([][]string, 0, len(e.policies))
	removed := false

	for _, p := range e.policies {
		if p[0] == sub && p[1] == obj && p[2] == act {
			removed = true
			continue
		}

		kept = append(kept, p)
	}

	e.policies = kept

	return removed, nil
}

func (e *fakeEnforcer) RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	kept := make([][]string, 0, len(e.policies))
	removed := false

	for _, p := range e.policies {
		match := true

		for i, want := range fieldValues {
			idx := fieldIndex + i

			if want == "" {
				continue
			}

			if idx >= len(p) || p[idx] != want {
				match = false
				break
			}
		}

		if match {
			removed = true
			continue
		}

		kept = append(kept, p)
	}

	e.policies = kept

	return removed, nil
}

func (e *fakeEnforcer) AddRoleForUser(user, role string) (bool, error) {
	for _, g := range e.groups {
		if g[0] == user && g[1] == role {
			return false, nil
		}
	}

	e.groups = append(e.groups, []string{user, role})

	return true, nil
}

func (e *fakeEnforcer) DeleteRoleForUser(user, role string) (bool, error) {
	kept := make([][]string, 0, len(e.groups))
	removed := false

	for _, g := range e.groups {
		if g[0] == user && g[1] == role {
			removed = true
			continue
		}

		kept = append(kept, g)
	}

	e.groups = kept

	return removed, nil
}

func (e *fakeEnforcer) DeleteRole(role string) (bool, error) {
	_, _ = e.RemoveFilteredPolicy(0, role)

	kept := make([][]string, 0, len(e.groups))
	for _, g := range e.groups {
		if g[0] == role || g[1] == role {
			continue
		}

		kept = append(kept, g)
	}

	e.groups = kept

	return true, nil
}

func (e *fakeEnforcer) SavePolicy() error {
	e.saved++

	return nil
}

// suite 应用层用例的装配结果。
type suite struct {
	db          *gorm.DB
	enforcer    *fakeEnforcer
	generator   *fakeMenuGenerator
	roles       *app.RoleService
	menus       *app.MenuService
	permissions *app.PermissionService
	inherits    *app.InheritService
	roleRepo    *infra.RoleRepository
}

// newSuite 用 sqlite 临时库 + fakeEnforcer 装配全部 rbac 应用服务。
func newSuite(t *testing.T) *suite {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rbac-app.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err := db.AutoMigrate(
			&po.Role{}, &po.Menu{}, &po.RoleMenu{}, &po.UserRole{}, &po.User{}, &po.APIResource{},
		)
		require.NoError(t, err, "migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	var (
		enforcer  = &fakeEnforcer{}
		roleRepo  = infra.NewRoleRepository(db)
		menuRepo  = infra.NewMenuRepository(db)
		rmRepo    = infra.NewRoleMenuRepository(db)
		urRepo    = infra.NewUserRoleRepository(db)
		resRepo   = infra.NewAPIResourceRepository(db)
		graph     = app.NewRoleGraph(enforcer)
		evaluator = infra.NewPermissionEvaluator()
		generator = &fakeMenuGenerator{}
		txm       = gormx.NewTxManager(db)
	)

	return &suite{
		db:          db,
		enforcer:    enforcer,
		generator:   generator,
		roles:       app.NewRoleService(txm, roleRepo, menuRepo, urRepo, enforcer, zap.NewNop()),
		menus:       app.NewMenuService(txm, roleRepo, menuRepo, rmRepo, urRepo, generator, zap.NewNop()),
		permissions: app.NewPermissionService(roleRepo, rmRepo, resRepo, enforcer, graph, evaluator, zap.NewNop()),
		inherits:    app.NewInheritService(roleRepo, rmRepo, enforcer, graph, zap.NewNop()),
		roleRepo:    roleRepo,
	}
}

func TestRoleServiceCreate(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	{
		err := s.roles.Create(ctx, app.CreateRoleParams{
			Name: "运营",
			Desc: "运营角色",
			Permissions: []domain.PermissionItem{
				{Resource: " /v1/admin/banner ", Action: " GET "},
			},
		})
		require.NoError(t, err, "create role: %v", err)
	}

	require.Len(t, s.enforcer.policies, 1, "unexpected policies: %+v", s.enforcer.policies)
	// 资源与方法必须去首尾空白后落库。
	{
		got := s.enforcer.policies[0]
		require.Equal(t, testV1AdminBanner, got[1], "unexpected policy: %+v", got)
		require.Equal(t, testGET, got[2], "unexpected policy: %+v", got)
	}

	{
		err := s.roles.Create(ctx, app.CreateRoleParams{Name: "运营"})
		require.ErrorIs(t, err, domain.ErrRoleNameExists, "expected ErrRoleNameExists, got %v", err)
	}

	// 权限参数不完整时报错文案必须原样下发。
	err := s.roles.Create(ctx, app.CreateRoleParams{
		Name:        "审核",
		Permissions: []domain.PermissionItem{{Resource: "", Action: testGET}},
	})
	require.ErrorIs(t, err, domain.ErrPermissionParamIncomplete, "expected ErrPermissionParamIncomplete, got %v", err)

	// 模板角色不存在时整个事务回滚，不应留下半成品角色。
	{
		err := s.roles.Create(ctx, app.CreateRoleParams{Name: "临时", CopyRoleID: 9999})
		require.ErrorIs(t, err, domain.ErrTemplateRoleNotFound, "expected ErrTemplateRoleNotFound, got %v", err)
	}

	{
		role, err := s.roleRepo.FindByName(ctx, "临时")
		require.NoError(t, err, "expected rollback, got role=%+v err=%v", role, err)
		require.Nil(t, role, "expected rollback, got role=%+v err=%v", role, err)
	}
}

func TestRoleServiceCreateFromTemplate(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	template := domain.NewRole("模板", "")
	{
		err := s.roleRepo.Create(ctx, template)
		require.NoError(t, err, "seed template role: %v", err)
	}

	{
		_, err := s.enforcer.AddPolicy("模板", "/v1/admin/**", "*")
		require.NoError(t, err, "seed policy: %v", err)
	}

	{
		err := s.roles.Create(ctx, app.CreateRoleParams{Name: "副本", CopyRoleID: template.ID})
		require.NoError(t, err, "create from template: %v", err)
	}

	copied, err := s.enforcer.GetPermissionsForUser("副本")
	require.NoError(t, err, "get permissions: %v", err)

	require.Len(t, copied, 1, "unexpected copied permissions: %+v", copied)
	require.Equal(t, "/v1/admin/**", copied[0][1], "unexpected copied permissions: %+v", copied)
	require.Equal(t, "*", copied[0][2], "unexpected copied permissions: %+v", copied)
}

func TestRoleServiceUpdateRenamesCasbin(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("旧名", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	{
		_, err := s.enforcer.AddPolicy("旧名", "/v1/admin/x", testGET)
		require.NoError(t, err, "seed policy: %v", err)
	}

	{
		_, err := s.enforcer.AddRoleForUser("user:1", "旧名")
		require.NoError(t, err, "seed grouping: %v", err)
	}

	newName := "新名"
	{
		err := s.roles.Update(ctx, app.UpdateRoleParams{RoleID: role.ID, Name: &newName})
		require.NoError(t, err, "update role: %v", err)
	}

	{
		rows, _ := s.enforcer.GetPermissionsForUser("新名")
		require.Len(t, rows, 1, "policies not migrated: %+v", s.enforcer.policies)
	}

	{
		rows, _ := s.enforcer.GetPermissionsForUser("旧名")
		require.Empty(t, rows, "old policies not removed: %+v", s.enforcer.policies)
	}

	{
		users, _ := s.enforcer.GetUsersForRole("新名")
		require.Len(t, users, 1, "grouping not migrated: %+v", s.enforcer.groups)
		require.Equal(t, "user:1", users[0], "grouping not migrated: %+v", s.enforcer.groups)
	}

	reloaded, err := s.roleRepo.FindByID(ctx, role.ID)
	require.NoError(t, err, "unexpected role after rename: %+v err=%v", reloaded, err)
	require.NotNil(t, reloaded, "unexpected role after rename: %+v err=%v", reloaded, err)
	require.Equal(t, reloaded.Name, newName, "unexpected role after rename: %+v err=%v", reloaded, err)

	// 无可更新字段时文案必须保持原样。
	{
		err := s.roles.Update(ctx, app.UpdateRoleParams{RoleID: role.ID})
		require.ErrorIs(t, err, domain.ErrNoUpdatableFields, "expected ErrNoUpdatableFields, got %v", err)
	}
}

func TestRoleServiceDeleteRejectsRoleInUse(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("在用角色", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	user := po.User{Username: "u1", Email: "u1@example.com", Password: "x"}
	{
		err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "seed user: %v", err)
	}

	{
		err := s.db.Create(&po.UserRole{UserID: user.ID, RoleID: role.ID}).Error
		require.NoError(t, err, "seed user role: %v", err)
	}

	{
		err := s.roles.Delete(ctx, role.ID)
		require.ErrorIs(t, err, domain.ErrRoleHasUsers, "expected ErrRoleHasUsers, got %v", err)
	}

	{
		err := s.roles.Delete(ctx, role.ID+999)
		require.ErrorIs(t, err, domain.ErrRoleNotFound, "expected ErrRoleNotFound, got %v", err)
	}
}

func TestPermissionServiceAssignRejectsCoveredPermission(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("受限角色", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	{
		_, err := s.enforcer.AddPolicy("受限角色", "/v1/admin/**", "*")
		require.NoError(t, err, "seed policy: %v", err)
	}

	err := s.permissions.Assign(ctx, app.AssignPermissionParams{
		RoleName:    "受限角色",
		Permissions: []domain.PermissionItem{{Resource: testV1AdminBanner, Action: "DELETE"}},
	})
	require.ErrorIs(t, err, domain.ErrPermissionAlreadyOwned, "expected ErrPermissionAlreadyOwned, got %v", err)

	// 列表内部冗余同样被拒。
	err = s.permissions.Assign(ctx, app.AssignPermissionParams{
		RoleName: "受限角色",
		Permissions: []domain.PermissionItem{
			{Resource: "/v1/common/**", Action: "*"},
			{Resource: "/v1/common/video", Action: testGET},
		},
	})
	require.ErrorIs(t, err, domain.ErrPermissionAlreadyOwned, "expected ErrPermissionAlreadyOwned for internal redundancy, got %v", err)

	{
		err := s.permissions.Assign(ctx, app.AssignPermissionParams{
			RoleName:    "不存在",
			Permissions: []domain.PermissionItem{{Resource: "/v1/x", Action: testGET}},
		})
		require.ErrorIs(t, err, domain.ErrRoleNotFound, "expected ErrRoleNotFound, got %v", err)
	}
}

func TestPermissionServiceRolePermissionsSources(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	for _, name := range []string{"超级管理员", testVIPRole, testRegularRole} {
		{
			err := s.roleRepo.Create(ctx, domain.NewRole(name, ""))
			require.NoError(t, err, "seed role %s: %v", name, err)
		}
	}

	// 超级管理员 → 大会员 → 普通用户
	{
		_, err := s.enforcer.AddRoleForUser("超级管理员", testVIPRole)
		require.NoError(t, err, "seed grouping: %v", err)
	}

	{
		_, err := s.enforcer.AddRoleForUser(testVIPRole, testRegularRole)
		require.NoError(t, err, "seed grouping: %v", err)
	}

	for _, p := range [][3]string{
		{"超级管理员", "/v1/admin/**", "*"},
		{testVIPRole, "/v1/vip/**", "*"},
		{testRegularRole, "/v1/common/**", "*"},
	} {
		{
			_, err := s.enforcer.AddPolicy(p[0], p[1], p[2])
			require.NoError(t, err, "seed policy: %v", err)
		}
	}

	{
		err := s.db.Create(&po.APIResource{Path: testV1AdminBanner, Method: "DELETE"}).Error
		require.NoError(t, err, "seed resource: %v", err)
	}

	resp, err := s.permissions.RolePermissions(ctx, "超级管理员")
	require.NoError(t, err, "role permissions: %v", err)

	sources := make(map[string]app.EffectivePermissionItem, len(resp.Effective))
	for _, item := range resp.Effective {
		sources[item.Owner] = item
	}

	{
		got := sources["超级管理员"]
		require.Equal(t, app.RolePermissionSourceDirect, got.Source, "unexpected direct source: %+v", got)
	}

	{
		got := sources[testVIPRole]
		require.Equal(t, app.RolePermissionSourceInheritDirect, got.Source, "unexpected inherit_direct source: %+v", got)
		require.Equal(t, testVIPRole, got.TopParent, "unexpected inherit_direct source: %+v", got)
	}

	{
		got := sources[testRegularRole]
		require.Equal(t, app.RolePermissionSourceInheritIndirect, got.Source, "unexpected inherit_indirect source: %+v", got)
		require.Equal(t, testVIPRole, got.TopParent, "unexpected inherit_indirect source: %+v", got)
	}

	require.Len(t, resp.Covered, 1, "unexpected covered list: %+v", resp.Covered)
	require.Equal(t, "DELETE:/v1/admin/banner", resp.Covered[0], "unexpected covered list: %+v", resp.Covered)
}

func TestInheritServiceAssignAndRemove(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子角色", "")
	parent := domain.NewRole("父角色", "")
	grand := domain.NewRole("祖角色", "")

	for _, role := range []*domain.Role{child, parent, grand} {
		{
			err := s.roleRepo.Create(ctx, role)
			require.NoError(t, err, "seed role: %v", err)
		}
	}

	{
		err := s.inherits.Assign(ctx, child.ID, child.ID)
		require.ErrorIs(t, err, domain.ErrParentRoleIsSelf, "expected ErrParentRoleIsSelf, got %v", err)
	}

	{
		err := s.inherits.Assign(ctx, parent.ID, grand.ID)
		require.NoError(t, err, "assign parent→grand: %v", err)
	}

	{
		err := s.inherits.Assign(ctx, child.ID, parent.ID)
		require.NoError(t, err, "assign child→parent: %v", err)
	}

	// 重复继承直接父：文案含「已直接继承」。
	err := s.inherits.Assign(ctx, child.ID, parent.ID)
	require.Error(t, err, "unexpected duplicate inherit error: %v", err)
	require.Equal(t, err.Error(), domain.FormatAlreadyInheritedMsg("父角色", domain.InheritKindDirect, "父角色"), "unexpected duplicate inherit error: %v", err)

	// 间接继承祖角色：拒绝并提示顶层父角色。
	err = s.inherits.Assign(ctx, child.ID, grand.ID)
	require.Error(t, err, "unexpected indirect inherit error: %v", err)
	require.Equal(t, err.Error(), domain.FormatAlreadyInheritedMsg("祖角色", domain.InheritKindIndirect, "父角色"), "unexpected indirect inherit error: %v", err)

	// 环检测：祖角色不能再继承子角色。
	{
		err := s.inherits.Assign(ctx, grand.ID, child.ID)
		require.ErrorIs(t, err, domain.ErrInheritCycle, "expected ErrInheritCycle, got %v", err)
	}

	list, err := s.inherits.List(ctx, child.ID)
	require.NoError(t, err, "inherit list: %v", err)

	require.Len(t, list.Direct, 1, "unexpected direct list: %+v", list.Direct)
	require.Equal(t, "父角色", list.Direct[0], "unexpected direct list: %+v", list.Direct)

	require.Len(t, list.Indirect, 1, "unexpected indirect list: %+v", list.Indirect)
	require.Equal(t, "祖角色", list.Indirect[0].Name, "unexpected indirect list: %+v", list.Indirect)
	require.Equal(t, "父角色", list.Indirect[0].Via, "unexpected indirect list: %+v", list.Indirect)

	// 解除间接继承应被拒绝并给出顶层父角色提示。
	err = s.inherits.Remove(ctx, child.ID, grand.ID)
	require.Error(t, err, "unexpected indirect unlink error: %v", err)
	require.Equal(t, err.Error(), domain.FormatIndirectUnlinkMsg("祖角色", "父角色"), "unexpected indirect unlink error: %v", err)

	{
		err := s.inherits.Remove(ctx, child.ID, parent.ID)
		require.NoError(t, err, "remove direct inherit: %v", err)
	}

	{
		parents, _ := s.enforcer.GetRolesForUser("子角色")
		require.Empty(t, parents, "expected inherit removed, got %+v", parents)
	}

	{
		err := s.inherits.Remove(ctx, child.ID, parent.ID)
		require.ErrorIs(t, err, domain.ErrNotDirectInherit, "expected ErrNotDirectInherit, got %v", err)
	}
}

func TestInheritServiceCopiesParentMenus(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子", "")
	parent := domain.NewRole("父", "")

	for _, role := range []*domain.Role{child, parent} {
		{
			err := s.roleRepo.Create(ctx, role)
			require.NoError(t, err, "seed role: %v", err)
		}
	}

	menu := po.Menu{Title: "菜单", Path: "/m", Name: "M", Component: "C"}
	{
		err := s.db.Create(&menu).Error
		require.NoError(t, err, "seed menu: %v", err)
	}

	{
		err := s.db.Create(&po.RoleMenu{RoleID: parent.ID, MenuID: menu.ID}).Error
		require.NoError(t, err, "seed parent role menu: %v", err)
	}

	{
		err := s.inherits.Assign(ctx, child.ID, parent.ID)
		require.NoError(t, err, "assign inherit: %v", err)
	}

	var rows []po.RoleMenu
	{
		err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error
		require.NoError(t, err, "load child role menus: %v", err)
	}

	require.Len(t, rows, 1, "unexpected inherited role menus: %+v", rows)
	require.Equal(t, rows[0].MenuID, menu.ID, "unexpected inherited role menus: %+v", rows)
	require.True(t, rows[0].IsInherited, "unexpected inherited role menus: %+v", rows)

	// 解除继承后继承菜单应被回收。
	{
		err := s.inherits.Remove(ctx, child.ID, parent.ID)
		require.NoError(t, err, "remove inherit: %v", err)
	}

	{
		err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error
		require.NoError(t, err, "reload child role menus: %v", err)
	}

	require.Empty(t, rows, "expected inherited menus to be dropped, got %+v", rows)
}

func TestPermissionServiceUpdateRolePermissionsBreakInheritance(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子角色", "")
	parent := domain.NewRole("父角色", "")

	for _, role := range []*domain.Role{child, parent} {
		{
			err := s.roleRepo.Create(ctx, role)
			require.NoError(t, err, "seed role: %v", err)
		}
	}

	{
		_, err := s.enforcer.AddRoleForUser("子角色", "父角色")
		require.NoError(t, err, "seed grouping: %v", err)
	}

	{
		_, err := s.enforcer.AddPolicy("父角色", "/v1/admin/**", "*")
		require.NoError(t, err, "seed policy: %v", err)
	}

	menu := po.Menu{Title: "继承菜单", Path: "/i", Name: "I", Component: "C"}
	{
		err := s.db.Create(&menu).Error
		require.NoError(t, err, "seed menu: %v", err)
	}

	{
		err := s.db.Create(&po.RoleMenu{RoleID: child.ID, MenuID: menu.ID, IsInherited: true}).Error
		require.NoError(t, err, "seed inherited role menu: %v", err)
	}

	// 未解除继承时，被继承权限覆盖的直接权限写入应被拒。
	err := s.permissions.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{
		RoleID:      child.ID,
		Permissions: []domain.PermissionItem{{Resource: testV1AdminBanner, Action: testGET}},
	})
	require.ErrorIs(t, err, domain.ErrPermissionAlreadyOwned, "expected ErrPermissionAlreadyOwned, got %v", err)

	// 解除继承后同样的写入应成功，并清掉继承关系与继承菜单。
	{
		err := s.permissions.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{
			RoleID:           child.ID,
			Permissions:      []domain.PermissionItem{{Resource: testV1AdminBanner, Action: testGET}},
			BreakInheritance: true,
		})
		require.NoError(t, err, "update with break inheritance: %v", err)
	}

	{
		parents, _ := s.enforcer.GetRolesForUser("子角色")
		require.Empty(t, parents, "expected inheritance cleared, got %+v", parents)
	}

	var rows []po.RoleMenu
	{
		err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error
		require.NoError(t, err, "load child role menus: %v", err)
	}

	require.Empty(t, rows, "expected inherited menus removed, got %+v", rows)

	direct, _ := s.enforcer.GetPermissionsForUser("子角色")
	require.Len(t, direct, 1, "unexpected direct permissions: %+v", direct)
	require.Equal(t, testV1AdminBanner, direct[0][1], "unexpected direct permissions: %+v", direct)
}

func TestRoleServiceUserRoleLifecycle(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	vip := domain.NewRole(testVIPRole, "")
	common := domain.NewRole(testRegularRole, "")

	for _, role := range []*domain.Role{vip, common} {
		{
			err := s.roleRepo.Create(ctx, role)
			require.NoError(t, err, "seed role: %v", err)
		}
	}

	user := po.User{Username: "u2", Email: "u2@example.com", Password: "x"}
	{
		err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "seed user: %v", err)
	}

	{
		err := s.roles.AssignUserRoles(ctx, user.ID, []uint{vip.ID})
		require.NoError(t, err, "assign user roles: %v", err)
	}

	{
		roles, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID))
		require.Len(t, roles, 1, "unexpected casbin grouping: %+v", s.enforcer.groups)
		require.Equal(t, testVIPRole, roles[0], "unexpected casbin grouping: %+v", s.enforcer.groups)
	}

	// 重复分配同一角色时提示首个重复角色名。
	err := s.roles.AssignUserRoles(ctx, user.ID, []uint{vip.ID})
	require.Error(t, err, "unexpected duplicate assign error: %v", err)
	require.Equal(t, "部分角色已分配: 大会员", err.Error(), "unexpected duplicate assign error: %v", err)

	{
		err := s.roles.AssignUserRoles(ctx, user.ID+999, []uint{vip.ID})
		require.ErrorIs(t, err, domain.ErrUserNotFound, "expected ErrUserNotFound, got %v", err)
	}

	{
		err := s.roles.AssignUserRoles(ctx, user.ID, []uint{9999})
		require.ErrorIs(t, err, domain.ErrSomeRolesNotFound, "expected ErrSomeRolesNotFound, got %v", err)
	}

	{
		err := s.roles.ReplaceUserRoles(ctx, user.ID, []uint{common.ID})
		require.NoError(t, err, "replace user roles: %v", err)
	}

	roles, err := s.roles.UserRoles(ctx, user.ID)
	require.NoError(t, err, "user roles: %v", err)

	require.Len(t, roles, 1, "unexpected user roles: %+v", roles)
	require.Equal(t, testRegularRole, roles[0].Name, "unexpected user roles: %+v", roles)

	{
		groups, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID))
		require.Len(t, groups, 1, "unexpected casbin grouping after replace: %+v", s.enforcer.groups)
		require.Equal(t, testRegularRole, groups[0], "unexpected casbin grouping after replace: %+v", s.enforcer.groups)
	}

	{
		err := s.roles.RemoveUserRoles(ctx, user.ID, []uint{common.ID})
		require.NoError(t, err, "remove user roles: %v", err)
	}

	{
		groups, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID))
		require.Empty(t, groups, "expected empty grouping, got %+v", s.enforcer.groups)
	}
}
