package rbac_test

import (
	"context"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	"f_ccg_video_back_ddd/pkg/casbins"

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
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(
		&po.Role{}, &po.Menu{}, &po.RoleMenu{}, &po.UserRole{}, &po.User{}, &po.ApiResource{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	var (
		enforcer  = &fakeEnforcer{}
		roleRepo  = infra.NewRoleRepository(db)
		menuRepo  = infra.NewMenuRepository(db)
		rmRepo    = infra.NewRoleMenuRepository(db)
		urRepo    = infra.NewUserRoleRepository(db)
		resRepo   = infra.NewApiResourceRepository(db)
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
	s := newSuite(t)
	ctx := context.Background()

	if err := s.roles.Create(ctx, app.CreateRoleParams{
		Name: "运营",
		Desc: "运营角色",
		Permissions: []domain.PermissionItem{
			{Resource: " /v1/admin/banner ", Action: " GET "},
		},
	}); err != nil {
		t.Fatalf("create role: %v", err)
	}

	if len(s.enforcer.policies) != 1 {
		t.Fatalf("unexpected policies: %+v", s.enforcer.policies)
	}
	// 资源与方法必须去首尾空白后落库。
	if got := s.enforcer.policies[0]; got[1] != "/v1/admin/banner" || got[2] != "GET" {
		t.Fatalf("unexpected policy: %+v", got)
	}

	if err := s.roles.Create(ctx, app.CreateRoleParams{Name: "运营"}); err != domain.ErrRoleNameExists {
		t.Fatalf("expected ErrRoleNameExists, got %v", err)
	}

	// 权限参数不完整时报错文案必须原样下发。
	err := s.roles.Create(ctx, app.CreateRoleParams{
		Name:        "审核",
		Permissions: []domain.PermissionItem{{Resource: "", Action: "GET"}},
	})
	if err != domain.ErrPermissionParamIncomplete {
		t.Fatalf("expected ErrPermissionParamIncomplete, got %v", err)
	}

	// 模板角色不存在时整个事务回滚，不应留下半成品角色。
	if err := s.roles.Create(ctx, app.CreateRoleParams{Name: "临时", CopyRoleID: 9999}); err != domain.ErrTemplateRoleNotFound {
		t.Fatalf("expected ErrTemplateRoleNotFound, got %v", err)
	}

	if role, err := s.roleRepo.FindByName(ctx, "临时"); err != nil || role != nil {
		t.Fatalf("expected rollback, got role=%+v err=%v", role, err)
	}
}

func TestRoleServiceCreateFromTemplate(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	template := domain.NewRole("模板", "")
	if err := s.roleRepo.Create(ctx, template); err != nil {
		t.Fatalf("seed template role: %v", err)
	}

	if _, err := s.enforcer.AddPolicy("模板", "/v1/admin/**", "*"); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	if err := s.roles.Create(ctx, app.CreateRoleParams{Name: "副本", CopyRoleID: template.ID}); err != nil {
		t.Fatalf("create from template: %v", err)
	}

	copied, err := s.enforcer.GetPermissionsForUser("副本")
	if err != nil {
		t.Fatalf("get permissions: %v", err)
	}

	if len(copied) != 1 || copied[0][1] != "/v1/admin/**" || copied[0][2] != "*" {
		t.Fatalf("unexpected copied permissions: %+v", copied)
	}
}

func TestRoleServiceUpdateRenamesCasbin(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("旧名", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	if _, err := s.enforcer.AddPolicy("旧名", "/v1/admin/x", "GET"); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	if _, err := s.enforcer.AddRoleForUser("user:1", "旧名"); err != nil {
		t.Fatalf("seed grouping: %v", err)
	}

	newName := "新名"
	if err := s.roles.Update(ctx, app.UpdateRoleParams{RoleID: role.ID, Name: &newName}); err != nil {
		t.Fatalf("update role: %v", err)
	}

	if rows, _ := s.enforcer.GetPermissionsForUser("新名"); len(rows) != 1 {
		t.Fatalf("policies not migrated: %+v", s.enforcer.policies)
	}

	if rows, _ := s.enforcer.GetPermissionsForUser("旧名"); len(rows) != 0 {
		t.Fatalf("old policies not removed: %+v", s.enforcer.policies)
	}

	if users, _ := s.enforcer.GetUsersForRole("新名"); len(users) != 1 || users[0] != "user:1" {
		t.Fatalf("grouping not migrated: %+v", s.enforcer.groups)
	}

	reloaded, err := s.roleRepo.FindByID(ctx, role.ID)
	if err != nil || reloaded == nil || reloaded.Name != newName {
		t.Fatalf("unexpected role after rename: %+v err=%v", reloaded, err)
	}

	// 无可更新字段时文案必须保持原样。
	if err := s.roles.Update(ctx, app.UpdateRoleParams{RoleID: role.ID}); err != domain.ErrNoUpdatableFields {
		t.Fatalf("expected ErrNoUpdatableFields, got %v", err)
	}
}

func TestRoleServiceDeleteRejectsRoleInUse(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("在用角色", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	user := po.User{Username: "u1", Email: "u1@example.com", Password: "x"}
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := s.db.Create(&po.UserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
		t.Fatalf("seed user role: %v", err)
	}

	if err := s.roles.Delete(ctx, role.ID); err != domain.ErrRoleHasUsers {
		t.Fatalf("expected ErrRoleHasUsers, got %v", err)
	}

	if err := s.roles.Delete(ctx, role.ID+999); err != domain.ErrRoleNotFound {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestPermissionServiceAssignRejectsCoveredPermission(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("受限角色", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	if _, err := s.enforcer.AddPolicy("受限角色", "/v1/admin/**", "*"); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	err := s.permissions.Assign(ctx, app.AssignPermissionParams{
		RoleName:    "受限角色",
		Permissions: []domain.PermissionItem{{Resource: "/v1/admin/banner", Action: "DELETE"}},
	})
	if err != domain.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}

	// 列表内部冗余同样被拒。
	err = s.permissions.Assign(ctx, app.AssignPermissionParams{
		RoleName: "受限角色",
		Permissions: []domain.PermissionItem{
			{Resource: "/v1/common/**", Action: "*"},
			{Resource: "/v1/common/video", Action: "GET"},
		},
	})
	if err != domain.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned for internal redundancy, got %v", err)
	}

	if err := s.permissions.Assign(ctx, app.AssignPermissionParams{
		RoleName:    "不存在",
		Permissions: []domain.PermissionItem{{Resource: "/v1/x", Action: "GET"}},
	}); err != domain.ErrRoleNotFound {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestPermissionServiceRolePermissionsSources(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	for _, name := range []string{"超级管理员", "大会员", "普通用户"} {
		if err := s.roleRepo.Create(ctx, domain.NewRole(name, "")); err != nil {
			t.Fatalf("seed role %s: %v", name, err)
		}
	}

	// 超级管理员 → 大会员 → 普通用户
	if _, err := s.enforcer.AddRoleForUser("超级管理员", "大会员"); err != nil {
		t.Fatalf("seed grouping: %v", err)
	}

	if _, err := s.enforcer.AddRoleForUser("大会员", "普通用户"); err != nil {
		t.Fatalf("seed grouping: %v", err)
	}

	for _, p := range [][3]string{
		{"超级管理员", "/v1/admin/**", "*"},
		{"大会员", "/v1/vip/**", "*"},
		{"普通用户", "/v1/common/**", "*"},
	} {
		if _, err := s.enforcer.AddPolicy(p[0], p[1], p[2]); err != nil {
			t.Fatalf("seed policy: %v", err)
		}
	}

	if err := s.db.Create(&po.ApiResource{Path: "/v1/admin/banner", Method: "DELETE"}).Error; err != nil {
		t.Fatalf("seed resource: %v", err)
	}

	resp, err := s.permissions.RolePermissions(ctx, "超级管理员")
	if err != nil {
		t.Fatalf("role permissions: %v", err)
	}

	sources := make(map[string]app.EffectivePermissionItem, len(resp.Effective))
	for _, item := range resp.Effective {
		sources[item.Owner] = item
	}

	if got := sources["超级管理员"]; got.Source != app.RolePermissionSourceDirect {
		t.Fatalf("unexpected direct source: %+v", got)
	}

	if got := sources["大会员"]; got.Source != app.RolePermissionSourceInheritDirect || got.TopParent != "大会员" {
		t.Fatalf("unexpected inherit_direct source: %+v", got)
	}

	if got := sources["普通用户"]; got.Source != app.RolePermissionSourceInheritIndirect || got.TopParent != "大会员" {
		t.Fatalf("unexpected inherit_indirect source: %+v", got)
	}

	if len(resp.Covered) != 1 || resp.Covered[0] != "DELETE:/v1/admin/banner" {
		t.Fatalf("unexpected covered list: %+v", resp.Covered)
	}
}

func TestInheritServiceAssignAndRemove(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子角色", "")
	parent := domain.NewRole("父角色", "")
	grand := domain.NewRole("祖角色", "")

	for _, role := range []*domain.Role{child, parent, grand} {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	if err := s.inherits.Assign(ctx, child.ID, child.ID); err != domain.ErrParentRoleIsSelf {
		t.Fatalf("expected ErrParentRoleIsSelf, got %v", err)
	}

	if err := s.inherits.Assign(ctx, parent.ID, grand.ID); err != nil {
		t.Fatalf("assign parent→grand: %v", err)
	}

	if err := s.inherits.Assign(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("assign child→parent: %v", err)
	}

	// 重复继承直接父：文案含「已直接继承」。
	err := s.inherits.Assign(ctx, child.ID, parent.ID)
	if err == nil || err.Error() != domain.FormatAlreadyInheritedMsg("父角色", domain.InheritKindDirect, "父角色") {
		t.Fatalf("unexpected duplicate inherit error: %v", err)
	}

	// 间接继承祖角色：拒绝并提示顶层父角色。
	err = s.inherits.Assign(ctx, child.ID, grand.ID)
	if err == nil || err.Error() != domain.FormatAlreadyInheritedMsg("祖角色", domain.InheritKindIndirect, "父角色") {
		t.Fatalf("unexpected indirect inherit error: %v", err)
	}

	// 环检测：祖角色不能再继承子角色。
	if err := s.inherits.Assign(ctx, grand.ID, child.ID); err != domain.ErrInheritCycle {
		t.Fatalf("expected ErrInheritCycle, got %v", err)
	}

	list, err := s.inherits.List(ctx, child.ID)
	if err != nil {
		t.Fatalf("inherit list: %v", err)
	}

	if len(list.Direct) != 1 || list.Direct[0] != "父角色" {
		t.Fatalf("unexpected direct list: %+v", list.Direct)
	}

	if len(list.Indirect) != 1 || list.Indirect[0].Name != "祖角色" || list.Indirect[0].Via != "父角色" {
		t.Fatalf("unexpected indirect list: %+v", list.Indirect)
	}

	// 解除间接继承应被拒绝并给出顶层父角色提示。
	err = s.inherits.Remove(ctx, child.ID, grand.ID)
	if err == nil || err.Error() != domain.FormatIndirectUnlinkMsg("祖角色", "父角色") {
		t.Fatalf("unexpected indirect unlink error: %v", err)
	}

	if err := s.inherits.Remove(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("remove direct inherit: %v", err)
	}

	if parents, _ := s.enforcer.GetRolesForUser("子角色"); len(parents) != 0 {
		t.Fatalf("expected inherit removed, got %+v", parents)
	}

	if err := s.inherits.Remove(ctx, child.ID, parent.ID); err != domain.ErrNotDirectInherit {
		t.Fatalf("expected ErrNotDirectInherit, got %v", err)
	}
}

func TestInheritServiceCopiesParentMenus(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子", "")
	parent := domain.NewRole("父", "")

	for _, role := range []*domain.Role{child, parent} {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	menu := po.Menu{Title: "菜单", Path: "/m", Name: "M", Component: "C"}
	if err := s.db.Create(&menu).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	if err := s.db.Create(&po.RoleMenu{RoleID: parent.ID, MenuID: menu.ID}).Error; err != nil {
		t.Fatalf("seed parent role menu: %v", err)
	}

	if err := s.inherits.Assign(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("assign inherit: %v", err)
	}

	var rows []po.RoleMenu
	if err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error; err != nil {
		t.Fatalf("load child role menus: %v", err)
	}

	if len(rows) != 1 || rows[0].MenuID != menu.ID || !rows[0].IsInherited {
		t.Fatalf("unexpected inherited role menus: %+v", rows)
	}

	// 解除继承后继承菜单应被回收。
	if err := s.inherits.Remove(ctx, child.ID, parent.ID); err != nil {
		t.Fatalf("remove inherit: %v", err)
	}

	if err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error; err != nil {
		t.Fatalf("reload child role menus: %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected inherited menus to be dropped, got %+v", rows)
	}
}

func TestPermissionServiceUpdateRolePermissionsBreakInheritance(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	child := domain.NewRole("子角色", "")
	parent := domain.NewRole("父角色", "")

	for _, role := range []*domain.Role{child, parent} {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	if _, err := s.enforcer.AddRoleForUser("子角色", "父角色"); err != nil {
		t.Fatalf("seed grouping: %v", err)
	}

	if _, err := s.enforcer.AddPolicy("父角色", "/v1/admin/**", "*"); err != nil {
		t.Fatalf("seed policy: %v", err)
	}

	menu := po.Menu{Title: "继承菜单", Path: "/i", Name: "I", Component: "C"}
	if err := s.db.Create(&menu).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	if err := s.db.Create(&po.RoleMenu{RoleID: child.ID, MenuID: menu.ID, IsInherited: true}).Error; err != nil {
		t.Fatalf("seed inherited role menu: %v", err)
	}

	// 未解除继承时，被继承权限覆盖的直接权限写入应被拒。
	err := s.permissions.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{
		RoleID:      child.ID,
		Permissions: []domain.PermissionItem{{Resource: "/v1/admin/banner", Action: "GET"}},
	})
	if err != domain.ErrPermissionAlreadyOwned {
		t.Fatalf("expected ErrPermissionAlreadyOwned, got %v", err)
	}

	// 解除继承后同样的写入应成功，并清掉继承关系与继承菜单。
	if err := s.permissions.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{
		RoleID:           child.ID,
		Permissions:      []domain.PermissionItem{{Resource: "/v1/admin/banner", Action: "GET"}},
		BreakInheritance: true,
	}); err != nil {
		t.Fatalf("update with break inheritance: %v", err)
	}

	if parents, _ := s.enforcer.GetRolesForUser("子角色"); len(parents) != 0 {
		t.Fatalf("expected inheritance cleared, got %+v", parents)
	}

	var rows []po.RoleMenu
	if err := s.db.Where("role_id = ?", child.ID).Find(&rows).Error; err != nil {
		t.Fatalf("load child role menus: %v", err)
	}

	if len(rows) != 0 {
		t.Fatalf("expected inherited menus removed, got %+v", rows)
	}

	direct, _ := s.enforcer.GetPermissionsForUser("子角色")
	if len(direct) != 1 || direct[0][1] != "/v1/admin/banner" {
		t.Fatalf("unexpected direct permissions: %+v", direct)
	}
}

func TestRoleServiceUserRoleLifecycle(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	vip := domain.NewRole("大会员", "")
	common := domain.NewRole("普通用户", "")

	for _, role := range []*domain.Role{vip, common} {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			t.Fatalf("seed role: %v", err)
		}
	}

	user := po.User{Username: "u2", Email: "u2@example.com", Password: "x"}
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if err := s.roles.AssignUserRoles(ctx, user.ID, []uint{vip.ID}); err != nil {
		t.Fatalf("assign user roles: %v", err)
	}

	if roles, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID)); len(roles) != 1 || roles[0] != "大会员" {
		t.Fatalf("unexpected casbin grouping: %+v", s.enforcer.groups)
	}

	// 重复分配同一角色时提示首个重复角色名。
	err := s.roles.AssignUserRoles(ctx, user.ID, []uint{vip.ID})
	if err == nil || err.Error() != "部分角色已分配: 大会员" {
		t.Fatalf("unexpected duplicate assign error: %v", err)
	}

	if err := s.roles.AssignUserRoles(ctx, user.ID+999, []uint{vip.ID}); err != domain.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	if err := s.roles.AssignUserRoles(ctx, user.ID, []uint{9999}); err != domain.ErrSomeRolesNotFound {
		t.Fatalf("expected ErrSomeRolesNotFound, got %v", err)
	}

	if err := s.roles.ReplaceUserRoles(ctx, user.ID, []uint{common.ID}); err != nil {
		t.Fatalf("replace user roles: %v", err)
	}

	roles, err := s.roles.UserRoles(ctx, user.ID)
	if err != nil {
		t.Fatalf("user roles: %v", err)
	}

	if len(roles) != 1 || roles[0].Name != "普通用户" {
		t.Fatalf("unexpected user roles: %+v", roles)
	}

	if groups, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID)); len(groups) != 1 || groups[0] != "普通用户" {
		t.Fatalf("unexpected casbin grouping after replace: %+v", s.enforcer.groups)
	}

	if err := s.roles.RemoveUserRoles(ctx, user.ID, []uint{common.ID}); err != nil {
		t.Fatalf("remove user roles: %v", err)
	}

	if groups, _ := s.enforcer.GetRolesForUser(casbins.GetUserSub(user.ID)); len(groups) != 0 {
		t.Fatalf("expected empty grouping, got %+v", s.enforcer.groups)
	}
}
