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

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fixture 一套 sqlite 临时库 + 真实仓储（测试不走 wire）。
type fixture struct {
	db        *gorm.DB
	roles     *infra.RoleRepository
	menus     *infra.MenuRepository
	roleMenus *infra.RoleMenuRepository
	userRoles *infra.UserRoleRepository
	resources *infra.ApiResourceRepository
}

// newFixture 建库并迁移 rbac 相关表。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rbac-test.db")), &gorm.Config{
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

	return &fixture{
		db:        db,
		roles:     infra.NewRoleRepository(db),
		menus:     infra.NewMenuRepository(db),
		roleMenus: infra.NewRoleMenuRepository(db),
		userRoles: infra.NewUserRoleRepository(db),
		resources: infra.NewApiResourceRepository(db),
	}
}

// seedUser 直接写用户行，跳过 po.User 的钩子（本测只关心 user_roles）。
func (f *fixture) seedUser(t *testing.T, username string, currentRoleID uint) uint {
	t.Helper()

	user := po.User{
		Username:      username,
		Email:         username + "@example.com",
		Password:      "x",
		CurrentRoleID: currentRoleID,
	}
	if err := f.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	return user.ID
}

func TestRoleAndMenuRepositories(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	admin := domain.NewRole("超级管理员", "全部权限")
	if err := f.roles.Create(ctx, admin); err != nil {
		t.Fatalf("create role: %v", err)
	}

	if admin.ID == 0 {
		t.Fatal("expected role id to be populated")
	}

	parent := domain.NewMenu("系统权限", "System", "", "icon", "/system", "System", "Layout", false, nil, 1)
	if err := f.menus.Create(ctx, parent); err != nil {
		t.Fatalf("create parent menu: %v", err)
	}

	child := domain.NewMenu("角色管理", "Roles", "", "icon", "roles", "RoleManagement", "views/roles", true, &parent.ID, 2)
	if err := f.menus.Create(ctx, child); err != nil {
		t.Fatalf("create child menu: %v", err)
	}

	// 直连 + 继承两类关联，验证 is_inherited 过滤。
	if err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewDirectRoleMenus(admin.ID, []*domain.Menu{parent})); err != nil {
		t.Fatalf("create direct role menu: %v", err)
	}

	if err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(admin.ID, []uint{child.ID})); err != nil {
		t.Fatalf("create inherited role menu: %v", err)
	}

	direct, err := f.menus.ListByRoleInherited(ctx, admin.ID, false)
	if err != nil {
		t.Fatalf("list direct menus: %v", err)
	}

	if len(direct) != 1 || direct[0].ID != parent.ID {
		t.Fatalf("unexpected direct menus: %+v", direct)
	}

	inherited, err := f.menus.ListByRoleInherited(ctx, admin.ID, true)
	if err != nil {
		t.Fatalf("list inherited menus: %v", err)
	}

	if len(inherited) != 1 || inherited[0].ID != child.ID {
		t.Fatalf("unexpected inherited menus: %+v", inherited)
	}

	all, err := f.menus.ListByRole(ctx, admin.ID)
	if err != nil {
		t.Fatalf("list role menus: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 menus for role, got %d", len(all))
	}

	inheritedIDs, err := f.roleMenus.InheritedMenuIDs(ctx, admin.ID)
	if err != nil {
		t.Fatalf("inherited menu ids: %v", err)
	}

	if _, ok := inheritedIDs[child.ID]; !ok || len(inheritedIDs) != 1 {
		t.Fatalf("unexpected inherited ids: %+v", inheritedIDs)
	}

	distinct, err := f.roleMenus.DistinctMenuIDsByRoles(ctx, []uint{admin.ID})
	if err != nil {
		t.Fatalf("distinct menu ids: %v", err)
	}

	if len(distinct) != 2 {
		t.Fatalf("expected 2 distinct menu ids, got %d", len(distinct))
	}

	// 角色列表应带出已绑定菜单（响应 JSON 依赖该预加载）。
	list, err := f.roles.ListWithMenus(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}

	if len(list) != 1 || len(list[0].Menus) != 2 {
		t.Fatalf("unexpected role list: %+v", list)
	}

	// 菜单更新：清空父菜单 + 改标题。
	title := "角色与权限"
	if err := f.menus.UpdateFields(ctx, child.ID, domain.MenuFieldUpdate{
		Title:       &title,
		ClearParent: true,
	}); err != nil {
		t.Fatalf("update menu: %v", err)
	}

	reloaded, err := f.menus.FindByID(ctx, child.ID)
	if err != nil || reloaded == nil {
		t.Fatalf("reload menu: %v", err)
	}

	if reloaded.Title != title || reloaded.ParentID != nil {
		t.Fatalf("unexpected menu after update: %+v", reloaded)
	}

	if err := gormx.NewTxManager(f.db).Do(ctx, func(txCtx context.Context) error {
		if err := f.roles.DeleteBindings(txCtx, admin.ID); err != nil {
			return err
		}
		return f.roles.DeleteByID(txCtx, admin.ID)
	}); err != nil {
		t.Fatalf("delete role: %v", err)
	}

	remaining, err := f.roleMenus.MenuIDsByRole(ctx, admin.ID)
	if err != nil {
		t.Fatalf("menu ids after delete: %v", err)
	}

	if len(remaining) != 0 {
		t.Fatalf("expected role_menus to be cleaned up, got %+v", remaining)
	}
}

func TestUserRoleRepository(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	vip := domain.NewRole("大会员", "")
	common := domain.NewRole("普通用户", "")

	for _, role := range []*domain.Role{vip, common} {
		if err := f.roles.Create(ctx, role); err != nil {
			t.Fatalf("create role: %v", err)
		}
	}

	userID := f.seedUser(t, "rbac_user", 0)

	// 无绑定关系时 GORM 预加载给出空切片，响应体为 data:[]，与原实现一致。
	roles, err := f.userRoles.RolesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("roles by user: %v", err)
	}

	if len(roles) != 0 {
		t.Fatalf("expected no roles for unbound user, got %+v", roles)
	}

	if _, err := f.userRoles.RolesByUser(ctx, userID+999); err != domain.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	if err := f.userRoles.AppendRoles(ctx, userID, []*domain.Role{vip}); err != nil {
		t.Fatalf("append roles: %v", err)
	}

	// 重复 append 走 ON CONFLICT DO NOTHING，不应报错也不应重复插入。
	if err := f.userRoles.AppendRoles(ctx, userID, []*domain.Role{vip}); err != nil {
		t.Fatalf("append roles twice: %v", err)
	}

	count, err := f.roles.UserCount(ctx, vip.ID)
	if err != nil {
		t.Fatalf("user count: %v", err)
	}

	if count != 1 {
		t.Fatalf("user count = %d, want 1", count)
	}

	if err := f.userRoles.ReplaceRoles(ctx, userID, []*domain.Role{common}); err != nil {
		t.Fatalf("replace roles: %v", err)
	}

	roles, err = f.userRoles.RolesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("roles by user after replace: %v", err)
	}

	if len(roles) != 1 || roles[0].Name != "普通用户" {
		t.Fatalf("unexpected roles after replace: %+v", roles)
	}

	if err := f.userRoles.RemoveRoles(ctx, userID, []*domain.Role{common}); err != nil {
		t.Fatalf("remove roles: %v", err)
	}

	roles, err = f.userRoles.RolesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("roles by user after remove: %v", err)
	}

	if len(roles) != 0 {
		t.Fatalf("expected no roles after remove, got %+v", roles)
	}

	// 用户 open_id 不应被角色绑定操作改写（原实现走 Association 会 Save users 行）。
	var stored po.User
	if err := f.db.First(&stored, userID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if stored.Username != "rbac_user" {
		t.Fatalf("unexpected user row after role writes: %+v", stored)
	}
}

func TestApiResourceRepository(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	item := &domain.ApiResource{Path: "/v1/admin/rbac/roles", Method: "GET", Summary: "GET /v1/admin/rbac/roles"}
	if err := f.resources.Create(ctx, item); err != nil {
		t.Fatalf("create resource: %v", err)
	}

	summary := "角色列表"
	tags := []byte(`["rbac"]`)

	if err := f.resources.UpdateSummaryAndTags(ctx, item.ID, &summary, &tags); err != nil {
		t.Fatalf("update resource: %v", err)
	}

	list, err := f.resources.ListOrdered(ctx)
	if err != nil {
		t.Fatalf("list resources: %v", err)
	}

	if len(list) != 1 || list[0].Summary != summary {
		t.Fatalf("unexpected resource list: %+v", list)
	}

	if got := domain.DecodeResourceTags(list[0].Tags); len(got) != 1 || got[0] != "rbac" {
		t.Fatalf("unexpected tags: %+v", got)
	}

	if err := f.resources.DeleteByID(ctx, item.ID); err != nil {
		t.Fatalf("delete resource: %v", err)
	}

	list, err = f.resources.ListAll(ctx)
	if err != nil {
		t.Fatalf("list all resources: %v", err)
	}

	if len(list) != 0 {
		t.Fatalf("expected empty resource table, got %+v", list)
	}
}

func TestMenuServiceAssignAndRead(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	role := domain.NewRole("运营", "")
	if err := f.roles.Create(ctx, role); err != nil {
		t.Fatalf("create role: %v", err)
	}

	first := domain.NewMenu("内容管理", "Content", "", "", "/content", "Content", "Layout", false, nil, 1)
	second := domain.NewMenu("稿件审核", "Review", "", "", "review", "SubmissionReview", "views/review", false, nil, 2)

	for _, menu := range []*domain.Menu{first, second} {
		if err := f.menus.Create(ctx, menu); err != nil {
			t.Fatalf("create menu: %v", err)
		}
	}

	second.ParentID = &first.ID
	if err := f.menus.UpdateFields(ctx, second.ID, domain.MenuFieldUpdate{ParentID: &first.ID}); err != nil {
		t.Fatalf("set parent: %v", err)
	}

	svc := app.NewMenuService(
		gormx.NewTxManager(f.db),
		f.roles,
		f.menus,
		f.roleMenus,
		f.userRoles,
		nil,
		zap.NewNop(),
	)

	// 继承菜单先落一条，AssignRoleMenus 必须把它从入参里过滤掉且不删除它。
	if err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(role.ID, []uint{second.ID})); err != nil {
		t.Fatalf("seed inherited menu: %v", err)
	}

	if err := svc.AssignRoleMenus(ctx, role.ID, []uint{first.ID, second.ID, first.ID}); err != nil {
		t.Fatalf("assign role menus: %v", err)
	}

	resp, err := svc.RoleMenus(ctx, role.ID)
	if err != nil {
		t.Fatalf("role menus: %v", err)
	}

	if len(resp.DirectMenus) != 1 || resp.DirectMenus[0].ID != first.ID {
		t.Fatalf("unexpected direct menu tree: %+v", resp.DirectMenus)
	}

	if len(resp.InheritMenus) != 1 || resp.InheritMenus[0].ID != second.ID {
		t.Fatalf("unexpected inherit menu tree: %+v", resp.InheritMenus)
	}

	// 当前角色为 0 的用户拿到空列表（而不是 null）。
	noRoleUser := f.seedUser(t, "no_role", 0)

	menus, err := svc.UserMenus(ctx, noRoleUser)
	if err != nil {
		t.Fatalf("user menus: %v", err)
	}

	if menus == nil || len(menus) != 0 {
		t.Fatalf("expected empty non-nil slice, got %+v", menus)
	}

	// 设置当前角色后应拿到菜单树（父子已挂接）。
	withRoleUser := f.seedUser(t, "with_role", role.ID)

	menus, err = svc.UserMenus(ctx, withRoleUser)
	if err != nil {
		t.Fatalf("user menus with role: %v", err)
	}

	if len(menus) != 1 || menus[0].ID != first.ID || len(menus[0].Children) != 1 {
		t.Fatalf("unexpected user menu tree: %+v", menus)
	}

	if _, err := svc.RoleMenus(ctx, role.ID+999); err != domain.ErrRoleNotFound {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}

	if _, err := svc.UserMenus(ctx, withRoleUser+999); err != domain.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
