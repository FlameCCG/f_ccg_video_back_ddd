package rbac_test

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/rbac"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"

	"go.uber.org/zap"
)

const (
	testV1AdminBanner = "/v1/admin/banner"
	testGET           = "GET"
	testVIPRole       = "大会员"
)

var errInjectedFailure = errors.New("injected operation failure")

type failingPolicy struct {
	app.Enforcer
	addCount, failAddAt int
	failSave            bool
}

func (e *failingPolicy) AddPolicy(s, o, a string) (bool, error) {
	e.addCount++
	if e.addCount == e.failAddAt {
		return false, errInjectedFailure
	}

	return e.Enforcer.AddPolicy(s, o, a)
}

func (e *failingPolicy) SavePolicy() error {
	if e.failSave {
		e.failSave = false
		return errInjectedFailure
	}

	return e.Enforcer.SavePolicy()
}

type failedCommit struct{ base tx.Manager }

func (m failedCommit) Do(ctx context.Context, fn func(context.Context) error) error {
	return m.base.Do(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}

		return errInjectedFailure
	})
}

func roleServiceFor(s *suite, e app.Enforcer, m tx.Manager) *app.RoleService {
	return app.NewRoleService(m, s.roleRepo, infra.NewMenuRepository(s.db), infra.NewUserRoleRepository(s.db), e, zap.NewNop())
}

func TestCreateRollsBackPartialPolicy(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}

	err := roleServiceFor(s, enforcer, gormx.NewTxManager(s.db)).Create(context.Background(), app.CreateRoleParams{Name: "partial", Permissions: []domain.PermissionItem{{Resource: "/one", Action: testGET}, {Resource: "/two", Action: "POST"}}})
	require.Error(t, err, "expected add failure")

	require.Empty(t, s.enforcer.policies, "partial policies remain: %v", s.enforcer.policies)
}

func TestPermissionReplacementRestoresPreviousSet(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("replace", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err)
	}

	_, _ = s.enforcer.AddPolicy(role.Name, "/old", testGET)
	previous, _ := s.enforcer.GetPolicy()
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}

	svc := app.NewPermissionService(s.roleRepo, infra.NewRoleMenuRepository(s.db), infra.NewAPIResourceRepository(s.db), enforcer, app.NewRoleGraph(enforcer), infra.NewPermissionEvaluator(), zap.NewNop())
	{
		err := svc.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{RoleID: role.ID, Permissions: []domain.PermissionItem{{Resource: "/new1", Action: testGET}, {Resource: "/new2", Action: "POST"}}})
		require.Error(t, err, "expected second add failure")
	}

	{
		got, _ := s.enforcer.GetPolicy()
		require.True(t, reflect.DeepEqual(got, previous), "policy = %v, want %v", got, previous)
	}
}

func TestPermissionReplacementRestoresInheritedMenus(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("break-inheritance", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err)
	}

	menus := infra.NewRoleMenuRepository(s.db)
	{
		err := menus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(role.ID, []uint{99}))
		require.NoError(t, err)
	}

	_, _ = s.enforcer.AddRoleForUser(role.Name, "parent")
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}

	svc := app.NewPermissionService(s.roleRepo, menus, infra.NewAPIResourceRepository(s.db), enforcer, app.NewRoleGraph(enforcer), infra.NewPermissionEvaluator(), zap.NewNop())
	{
		err := svc.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{RoleID: role.ID, BreakInheritance: true, Permissions: []domain.PermissionItem{{Resource: "/one", Action: testGET}, {Resource: "/two", Action: "POST"}}})
		require.Error(t, err, "expected failure")
	}

	inherited, err := menus.InheritedMenuIDs(ctx, role.ID)
	require.NoError(t, err)

	{
		_, ok := inherited[99]
		require.True(t, ok, "failed replacement permanently removed inherited menu")
	}

	require.Len(t, s.enforcer.groups, 1, "inheritance not restored: %v", s.enforcer.groups)
	require.Equal(t, "parent", s.enforcer.groups[0][1], "inheritance not restored: %v", s.enforcer.groups)
}

func TestUserRoleCommitFailureNeverChangesCasbin(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("grant", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err)
	}

	user := po.User{Username: "grant-user", Email: "grant@example.test"}
	{
		err := s.db.Create(&user).Error
		require.NoError(t, err)
	}

	svc := roleServiceFor(s, s.enforcer, failedCommit{gormx.NewTxManager(s.db)})
	{
		err := svc.AssignUserRoles(ctx, user.ID, []uint{role.ID})
		require.Error(t, err, "expected commit failure")
	}

	require.Empty(t, s.enforcer.groups, "casbin changed before commit: %+v", s.enforcer)
	require.Equal(t, 0, s.enforcer.saved, "casbin changed before commit: %+v", s.enforcer)

	var count int64
	s.db.Model(&po.UserRole{}).Where("user_id = ?", user.ID).Count(&count)

	require.EqualValues(t, 0, count, "database role committed: %d", count)
}

func TestUserRoleSaveFailureCompensatesBothStores(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("grant", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err)
	}

	user := po.User{Username: "save-user", Email: "save@example.test"}
	{
		err := s.db.Create(&user).Error
		require.NoError(t, err)
	}

	svc := roleServiceFor(s, &failingPolicy{Enforcer: s.enforcer, failSave: true}, gormx.NewTxManager(s.db))
	{
		err := svc.AssignUserRoles(ctx, user.ID, []uint{role.ID})
		require.Error(t, err, "expected save failure")
	}

	require.Empty(t, s.enforcer.groups, "grant remains in casbin: %v", s.enforcer.groups)

	roles, err := infra.NewUserRoleRepository(s.db).RolesByUser(ctx, user.ID)
	require.NoError(t, err, "grant remains in database: %v %v", roles, err)
	require.Empty(t, roles, "grant remains in database: %v %v", roles, err)
}

func TestDeletePolicyFailureLeavesRoleRetryable(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("delete", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err)
	}

	_, _ = s.enforcer.AddPolicy(role.Name, "/old", testGET)

	svc := roleServiceFor(s, &failingPolicy{Enforcer: s.enforcer, failSave: true}, gormx.NewTxManager(s.db))
	{
		err := svc.Delete(ctx, role.ID)
		require.Error(t, err, "expected save failure")
	}

	{
		got, err := s.roleRepo.FindByID(ctx, role.ID)
		require.NoError(t, err, "role lost before policy save: %v %v", got, err)
		require.NotNil(t, got, "role lost before policy save: %v %v", got, err)
	}

	require.Len(t, s.enforcer.policies, 1, "old policy was not restored: %v", s.enforcer.policies)

	{
		err := svc.Delete(ctx, role.ID)
		require.NoError(t, err, "retry: %v", err)
	}
}

type failingInheritedMenus struct{ domain.RoleMenuRepository }

func (failingInheritedMenus) DistinctMenuIDsByRole(context.Context, uint) ([]uint, error) {
	return []uint{1}, nil
}

func (failingInheritedMenus) MenuIDsByRole(context.Context, uint) ([]uint, error) { return nil, nil }

func (failingInheritedMenus) CreateIgnoreConflict(context.Context, []domain.RoleMenu) error {
	return errInjectedFailure
}

func TestInheritanceMenuFailureRemovesNewEdge(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()
	child := domain.NewRole("child", "")

	parent := domain.NewRole("parent", "")
	for _, role := range []*domain.Role{child, parent} {
		{
			err := s.roleRepo.Create(ctx, role)
			require.NoError(t, err)
		}
	}

	svc := app.NewInheritService(s.roleRepo, failingInheritedMenus{}, s.enforcer, app.NewRoleGraph(s.enforcer), zap.NewNop())
	{
		err := svc.Assign(ctx, child.ID, parent.ID)
		require.Error(t, err, "expected menu failure")
	}

	require.Empty(t, s.enforcer.groups, "failed inheritance remains effective: %v", s.enforcer.groups)

	{
		err := svc.Assign(ctx, child.ID, parent.ID)
		require.ErrorIs(t, err, domain.ErrInheritFailed, "retry must attempt menus again, got %v", err)
	}
}
