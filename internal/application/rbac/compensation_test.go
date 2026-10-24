package rbac_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	"go.uber.org/zap"
)

var injectedFailure = errors.New("injected operation failure")

type failingPolicy struct {
	app.Enforcer
	addCount, failAddAt int
	failSave            bool
}

func (e *failingPolicy) AddPolicy(s, o, a string) (bool, error) {
	e.addCount++
	if e.addCount == e.failAddAt {
		return false, injectedFailure
	}
	return e.Enforcer.AddPolicy(s, o, a)
}
func (e *failingPolicy) SavePolicy() error {
	if e.failSave {
		e.failSave = false
		return injectedFailure
	}
	return e.Enforcer.SavePolicy()
}

type failedCommit struct{ base tx.Manager }

func (m failedCommit) Do(ctx context.Context, fn func(context.Context) error) error {
	return m.base.Do(ctx, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return injectedFailure
	})
}

func roleServiceFor(s *suite, e app.Enforcer, m tx.Manager) *app.RoleService {
	return app.NewRoleService(m, s.roleRepo, infra.NewMenuRepository(s.db), infra.NewUserRoleRepository(s.db), e, zap.NewNop())
}

func TestCreateRollsBackPartialPolicy(t *testing.T) {
	s := newSuite(t)
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}
	err := roleServiceFor(s, enforcer, gormx.NewTxManager(s.db)).Create(context.Background(), app.CreateRoleParams{Name: "partial", Permissions: []domain.PermissionItem{{Resource: "/one", Action: "GET"}, {Resource: "/two", Action: "POST"}}})
	if err == nil {
		t.Fatal("expected add failure")
	}
	if len(s.enforcer.policies) != 0 {
		t.Fatalf("partial policies remain: %v", s.enforcer.policies)
	}
}

func TestPermissionReplacementRestoresPreviousSet(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	role := domain.NewRole("replace", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	_, _ = s.enforcer.AddPolicy(role.Name, "/old", "GET")
	previous, _ := s.enforcer.GetPolicy()
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}
	svc := app.NewPermissionService(s.roleRepo, infra.NewRoleMenuRepository(s.db), infra.NewApiResourceRepository(s.db), enforcer, app.NewRoleGraph(enforcer), infra.NewPermissionEvaluator(), zap.NewNop())
	if err := svc.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{RoleID: role.ID, Permissions: []domain.PermissionItem{{Resource: "/new1", Action: "GET"}, {Resource: "/new2", Action: "POST"}}}); err == nil {
		t.Fatal("expected second add failure")
	}
	if got, _ := s.enforcer.GetPolicy(); !reflect.DeepEqual(got, previous) {
		t.Fatalf("policy = %v, want %v", got, previous)
	}
}

func TestPermissionReplacementRestoresInheritedMenus(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	role := domain.NewRole("break-inheritance", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	menus := infra.NewRoleMenuRepository(s.db)
	if err := menus.CreateIgnoreConflict(ctx, domain.NewInheritedRoleMenus(role.ID, []uint{99})); err != nil {
		t.Fatal(err)
	}
	_, _ = s.enforcer.AddRoleForUser(role.Name, "parent")
	enforcer := &failingPolicy{Enforcer: s.enforcer, failAddAt: 2}
	svc := app.NewPermissionService(s.roleRepo, menus, infra.NewApiResourceRepository(s.db), enforcer, app.NewRoleGraph(enforcer), infra.NewPermissionEvaluator(), zap.NewNop())
	if err := svc.UpdateRolePermissions(ctx, app.UpdateRolePermissionsParams{RoleID: role.ID, BreakInheritance: true, Permissions: []domain.PermissionItem{{Resource: "/one", Action: "GET"}, {Resource: "/two", Action: "POST"}}}); err == nil {
		t.Fatal("expected failure")
	}
	inherited, err := menus.InheritedMenuIDs(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := inherited[99]; !ok {
		t.Fatal("failed replacement permanently removed inherited menu")
	}
	if len(s.enforcer.groups) != 1 || s.enforcer.groups[0][1] != "parent" {
		t.Fatalf("inheritance not restored: %v", s.enforcer.groups)
	}
}

func TestUserRoleCommitFailureNeverChangesCasbin(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	role := domain.NewRole("grant", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	user := po.User{Username: "grant-user", Email: "grant@example.test"}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	svc := roleServiceFor(s, s.enforcer, failedCommit{gormx.NewTxManager(s.db)})
	if err := svc.AssignUserRoles(ctx, user.ID, []uint{role.ID}); err == nil {
		t.Fatal("expected commit failure")
	}
	if len(s.enforcer.groups) != 0 || s.enforcer.saved != 0 {
		t.Fatalf("casbin changed before commit: %+v", s.enforcer)
	}
	var count int64
	s.db.Model(&po.UserRole{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Fatalf("database role committed: %d", count)
	}
}

func TestUserRoleSaveFailureCompensatesBothStores(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	role := domain.NewRole("grant", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	user := po.User{Username: "save-user", Email: "save@example.test"}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	svc := roleServiceFor(s, &failingPolicy{Enforcer: s.enforcer, failSave: true}, gormx.NewTxManager(s.db))
	if err := svc.AssignUserRoles(ctx, user.ID, []uint{role.ID}); err == nil {
		t.Fatal("expected save failure")
	}
	if len(s.enforcer.groups) != 0 {
		t.Fatalf("grant remains in casbin: %v", s.enforcer.groups)
	}
	roles, err := infra.NewUserRoleRepository(s.db).RolesByUser(ctx, user.ID)
	if err != nil || len(roles) != 0 {
		t.Fatalf("grant remains in database: %v %v", roles, err)
	}
}

func TestDeletePolicyFailureLeavesRoleRetryable(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	role := domain.NewRole("delete", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	_, _ = s.enforcer.AddPolicy(role.Name, "/old", "GET")
	svc := roleServiceFor(s, &failingPolicy{Enforcer: s.enforcer, failSave: true}, gormx.NewTxManager(s.db))
	if err := svc.Delete(ctx, role.ID); err == nil {
		t.Fatal("expected save failure")
	}
	if got, err := s.roleRepo.FindByID(ctx, role.ID); err != nil || got == nil {
		t.Fatalf("role lost before policy save: %v %v", got, err)
	}
	if len(s.enforcer.policies) != 1 {
		t.Fatalf("old policy was not restored: %v", s.enforcer.policies)
	}
	if err := svc.Delete(ctx, role.ID); err != nil {
		t.Fatalf("retry: %v", err)
	}
}

type failingInheritedMenus struct{ domain.RoleMenuRepository }

func (failingInheritedMenus) DistinctMenuIDsByRole(context.Context, uint) ([]uint, error) {
	return []uint{1}, nil
}
func (failingInheritedMenus) MenuIDsByRole(context.Context, uint) ([]uint, error) { return nil, nil }
func (failingInheritedMenus) CreateIgnoreConflict(context.Context, []domain.RoleMenu) error {
	return injectedFailure
}

func TestInheritanceMenuFailureRemovesNewEdge(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()
	child := domain.NewRole("child", "")
	parent := domain.NewRole("parent", "")
	for _, role := range []*domain.Role{child, parent} {
		if err := s.roleRepo.Create(ctx, role); err != nil {
			t.Fatal(err)
		}
	}
	svc := app.NewInheritService(s.roleRepo, failingInheritedMenus{}, s.enforcer, app.NewRoleGraph(s.enforcer), zap.NewNop())
	if err := svc.Assign(ctx, child.ID, parent.ID); err == nil {
		t.Fatal("expected menu failure")
	}
	if len(s.enforcer.groups) != 0 {
		t.Fatalf("failed inheritance remains effective: %v", s.enforcer.groups)
	}
	if err := svc.Assign(ctx, child.ID, parent.ID); !errors.Is(err, domain.ErrInheritFailed) {
		t.Fatalf("retry must attempt menus again, got %v", err)
	}
}
