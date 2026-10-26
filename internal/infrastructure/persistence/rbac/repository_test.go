package rbac_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
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

// fixture 一套 sqlite 临时库 + 真实仓储（测试不走 wire）。
type fixture struct {
	db        *gorm.DB
	roles     *infra.RoleRepository
	menus     *infra.MenuRepository
	roleMenus *infra.RoleMenuRepository
	userRoles *infra.UserRoleRepository
	resources *infra.APIResourceRepository
}

// newFixture 建库并迁移 rbac 相关表。
func newFixture(t *testing.T) *fixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rbac-test.db")), &gorm.Config{
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

	return &fixture{
		db:        db,
		roles:     infra.NewRoleRepository(db),
		menus:     infra.NewMenuRepository(db),
		roleMenus: infra.NewRoleMenuRepository(db),
		userRoles: infra.NewUserRoleRepository(db),
		resources: infra.NewAPIResourceRepository(db),
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
	{
		err := f.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "create user failed: %v", err)
	}

	return user.ID
}

func TestRoleAndMenuRepositories(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	admin := domain.NewRole("超级管理员", "全部权限")
	{
		err := f.roles.Create(ctx, admin)
		require.NoError(t, err, "create role: %v", err)
	}

	require.NotEqualValues(t, 0, admin.ID, "expected role id to be populated")

	parent := domain.NewMenu("系统权限", "System", "", "icon", "/system", "System", "Layout", false, nil, 1)
	{
		err := f.menus.Create(ctx, parent)
		require.NoError(t, err, "create parent menu: %v", err)
	}

	child := domain.NewMenu("角色管理", "Roles", "", "icon", "roles", "RoleManagement", "views/roles", true, &parent.ID, 2)
	{
		err := f.menus.Create(ctx, child)
		require.NoError(t, err, "create child menu: %v", err)
	}

	// 直连 + 继承两类关联，验证 is_inherited 过滤。
	{
		err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewDirectRoleMenus(admin.ID, []*domain.Menu{parent}))
		require.NoError(t, err, "create direct role menu: %v", err)
	}

	{
		err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(admin.ID, []uint{child.ID}))
		require.NoError(t, err, "create inherited role menu: %v", err)
	}

	direct, err := f.menus.ListByRoleInherited(ctx, admin.ID, false)
	require.NoError(t, err, "list direct menus: %v", err)

	require.Len(t, direct, 1, "unexpected direct menus: %+v", direct)
	require.Equal(t, direct[0].ID, parent.ID, "unexpected direct menus: %+v", direct)

	inherited, err := f.menus.ListByRoleInherited(ctx, admin.ID, true)
	require.NoError(t, err, "list inherited menus: %v", err)

	require.Len(t, inherited, 1, "unexpected inherited menus: %+v", inherited)
	require.Equal(t, inherited[0].ID, child.ID, "unexpected inherited menus: %+v", inherited)

	all, err := f.menus.ListByRole(ctx, admin.ID)
	require.NoError(t, err, "list role menus: %v", err)

	require.Len(t, all, 2, "expected 2 menus for role, got %d", len(all))

	inheritedIDs, err := f.roleMenus.InheritedMenuIDs(ctx, admin.ID)
	require.NoError(t, err, "inherited menu ids: %v", err)

	{
		_, ok := inheritedIDs[child.ID]
		require.True(t, ok, "unexpected inherited ids: %+v", inheritedIDs)
		require.Len(t, inheritedIDs, 1, "unexpected inherited ids: %+v", inheritedIDs)
	}

	distinct, err := f.roleMenus.DistinctMenuIDsByRoles(ctx, []uint{admin.ID})
	require.NoError(t, err, "distinct menu ids: %v", err)

	require.Len(t, distinct, 2, "expected 2 distinct menu ids, got %d", len(distinct))

	// 角色列表应带出已绑定菜单（响应 JSON 依赖该预加载）。
	list, err := f.roles.ListWithMenus(ctx)
	require.NoError(t, err, "list roles: %v", err)

	require.Len(t, list, 1, "unexpected role list: %+v", list)
	require.Len(t, list[0].Menus, 2, "unexpected role list: %+v", list)

	// 菜单更新：清空父菜单 + 改标题。
	title := "角色与权限"
	{
		err := f.menus.UpdateFields(ctx, child.ID, domain.MenuFieldUpdate{
			Title:       &title,
			ClearParent: true,
		})
		require.NoError(t, err, "update menu: %v", err)
	}

	reloaded, err := f.menus.FindByID(ctx, child.ID)
	require.NoError(t, err, "reload menu: %v", err)
	require.NotNil(t, reloaded, "reload menu: %v", err)

	require.Equal(t, reloaded.Title, title, "unexpected menu after update: %+v", reloaded)
	require.Nil(t, reloaded.ParentID, "unexpected menu after update: %+v", reloaded)

	{
		err := gormx.NewTxManager(f.db).Do(ctx, func(txCtx context.Context) error {
			if err := f.roles.DeleteBindings(txCtx, admin.ID); err != nil {
				return err
			}

			return f.roles.DeleteByID(txCtx, admin.ID)
		})
		require.NoError(t, err, "delete role: %v", err)
	}

	remaining, err := f.roleMenus.MenuIDsByRole(ctx, admin.ID)
	require.NoError(t, err, "menu ids after delete: %v", err)

	require.Empty(t, remaining, "expected role_menus to be cleaned up, got %+v", remaining)
}

func TestUserRoleRepository(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	vip := domain.NewRole("大会员", "")
	common := domain.NewRole("普通用户", "")

	for _, role := range []*domain.Role{vip, common} {
		{
			err := f.roles.Create(ctx, role)
			require.NoError(t, err, "create role: %v", err)
		}
	}

	userID := f.seedUser(t, "rbac_user", 0)

	// 无绑定关系时 GORM 预加载给出空切片，响应体为 data:[]，与原实现一致。
	roles, err := f.userRoles.RolesByUser(ctx, userID)
	require.NoError(t, err, "roles by user: %v", err)

	require.Empty(t, roles, "expected no roles for unbound user, got %+v", roles)

	{
		_, err := f.userRoles.RolesByUser(ctx, userID+999)
		require.ErrorIs(t, err, domain.ErrUserNotFound, "expected ErrUserNotFound, got %v", err)
	}

	{
		err := f.userRoles.AppendRoles(ctx, userID, []*domain.Role{vip})
		require.NoError(t, err, "append roles: %v", err)
	}

	// 重复 append 走 ON CONFLICT DO NOTHING，不应报错也不应重复插入。
	{
		err := f.userRoles.AppendRoles(ctx, userID, []*domain.Role{vip})
		require.NoError(t, err, "append roles twice: %v", err)
	}

	count, err := f.roles.UserCount(ctx, vip.ID)
	require.NoError(t, err, "user count: %v", err)

	require.EqualValues(t, 1, count, "user count = %d, want 1", count)

	{
		err := f.userRoles.ReplaceRoles(ctx, userID, []*domain.Role{common})
		require.NoError(t, err, "replace roles: %v", err)
	}

	roles, err = f.userRoles.RolesByUser(ctx, userID)
	require.NoError(t, err, "roles by user after replace: %v", err)

	require.Len(t, roles, 1, "unexpected roles after replace: %+v", roles)
	require.Equal(t, "普通用户", roles[0].Name, "unexpected roles after replace: %+v", roles)

	{
		err := f.userRoles.RemoveRoles(ctx, userID, []*domain.Role{common})
		require.NoError(t, err, "remove roles: %v", err)
	}

	roles, err = f.userRoles.RolesByUser(ctx, userID)
	require.NoError(t, err, "roles by user after remove: %v", err)

	require.Empty(t, roles, "expected no roles after remove, got %+v", roles)

	// 用户 open_id 不应被角色绑定操作改写（原实现走 Association 会 Save users 行）。
	var stored po.User
	{
		err := f.db.First(&stored, userID).Error
		require.NoError(t, err, "reload user: %v", err)
	}

	require.Equal(t, "rbac_user", stored.Username, "unexpected user row after role writes: %+v", stored)
}

func TestApiResourceRepository(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	item := &domain.APIResource{Path: "/v1/admin/rbac/roles", Method: testGET, Summary: "GET /v1/admin/rbac/roles"}
	{
		err := f.resources.Create(ctx, item)
		require.NoError(t, err, "create resource: %v", err)
	}

	summary := "角色列表"
	tags := []byte(`["rbac"]`)

	{
		err := f.resources.UpdateSummaryAndTags(ctx, item.ID, &summary, &tags)
		require.NoError(t, err, "update resource: %v", err)
	}

	list, err := f.resources.ListOrdered(ctx)
	require.NoError(t, err, "list resources: %v", err)

	require.Len(t, list, 1, "unexpected resource list: %+v", list)
	require.Equal(t, list[0].Summary, summary, "unexpected resource list: %+v", list)

	{
		got := domain.DecodeResourceTags(list[0].Tags)
		require.Len(t, got, 1, "unexpected tags: %+v", got)
		require.Equal(t, "rbac", got[0], "unexpected tags: %+v", got)
	}

	{
		err := f.resources.DeleteByID(ctx, item.ID)
		require.NoError(t, err, "delete resource: %v", err)
	}

	list, err = f.resources.ListAll(ctx)
	require.NoError(t, err, "list all resources: %v", err)

	require.Empty(t, list, "expected empty resource table, got %+v", list)
}

func TestMenuServiceAssignAndRead(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	role := domain.NewRole("运营", "")
	{
		err := f.roles.Create(ctx, role)
		require.NoError(t, err, "create role: %v", err)
	}

	first := domain.NewMenu("内容管理", "Content", "", "", "/content", "Content", "Layout", false, nil, 1)
	second := domain.NewMenu("稿件审核", "Review", "", "", "review", "SubmissionReview", "views/review", false, nil, 2)

	for _, menu := range []*domain.Menu{first, second} {
		{
			err := f.menus.Create(ctx, menu)
			require.NoError(t, err, "create menu: %v", err)
		}
	}

	second.ParentID = &first.ID
	{
		err := f.menus.UpdateFields(ctx, second.ID, domain.MenuFieldUpdate{ParentID: &first.ID})
		require.NoError(t, err, "set parent: %v", err)
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
	{
		err := f.roleMenus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(role.ID, []uint{second.ID}))
		require.NoError(t, err, "seed inherited menu: %v", err)
	}

	{
		err := svc.AssignRoleMenus(ctx, role.ID, []uint{first.ID, second.ID, first.ID})
		require.NoError(t, err, "assign role menus: %v", err)
	}

	resp, err := svc.RoleMenus(ctx, role.ID)
	require.NoError(t, err, "role menus: %v", err)

	require.Len(t, resp.DirectMenus, 1, "unexpected direct menu tree: %+v", resp.DirectMenus)
	require.Equal(t, resp.DirectMenus[0].ID, first.ID, "unexpected direct menu tree: %+v", resp.DirectMenus)

	require.Len(t, resp.InheritMenus, 1, "unexpected inherit menu tree: %+v", resp.InheritMenus)
	require.Equal(t, resp.InheritMenus[0].ID, second.ID, "unexpected inherit menu tree: %+v", resp.InheritMenus)

	// 当前角色为 0 的用户拿到空列表（而不是 null）。
	noRoleUser := f.seedUser(t, "no_role", 0)

	menus, err := svc.UserMenus(ctx, noRoleUser)
	require.NoError(t, err, "user menus: %v", err)

	require.NotNil(t, menus, "expected empty non-nil slice, got %+v", menus)
	require.Empty(t, menus, "expected empty non-nil slice, got %+v", menus)

	// 设置当前角色后应拿到菜单树（父子已挂接）。
	withRoleUser := f.seedUser(t, "with_role", role.ID)

	menus, err = svc.UserMenus(ctx, withRoleUser)
	require.NoError(t, err, "user menus with role: %v", err)

	require.Len(t, menus, 1, "unexpected user menu tree: %+v", menus)
	require.Equal(t, menus[0].ID, first.ID, "unexpected user menu tree: %+v", menus)
	require.Len(t, menus[0].Children, 1, "unexpected user menu tree: %+v", menus)

	{
		_, err := svc.RoleMenus(ctx, role.ID+999)
		require.ErrorIs(t, err, domain.ErrRoleNotFound, "expected ErrRoleNotFound, got %v", err)
	}

	{
		_, err := svc.UserMenus(ctx, withRoleUser+999)
		require.ErrorIs(t, err, domain.ErrUserNotFound, "expected ErrUserNotFound, got %v", err)
	}
}
