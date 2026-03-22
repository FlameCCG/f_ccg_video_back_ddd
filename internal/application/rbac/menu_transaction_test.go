package rbac_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"

	"go.uber.org/zap"
)

type failingMenuRemoval struct {
	rbacdomain.MenuRepository
	failure error
}

func (r failingMenuRemoval) DeleteByID(context.Context, uint) error {
	return r.failure
}

type failingRoleMenuInsert struct {
	rbacdomain.RoleMenuRepository
	failure error
}

func (r failingRoleMenuInsert) CreateIgnoreConflict(context.Context, []rbacdomain.RoleMenu) error {
	return r.failure
}

func newMenuTransactionSuite(t *testing.T) *suite {
	t.Helper()
	s := newSuite(t)
	for _, role := range []po.Role{
		{Model: po.Model{ID: 1}, Name: "target"},
		{Model: po.Model{ID: 2}, Name: "other"},
	} {
		if err := s.db.Create(&role).Error; err != nil {
			t.Fatal(err)
		}
	}
	for id := uint(1); id <= 4; id++ {
		if err := s.db.Create(&po.Menu{Model: po.Model{ID: id}, Title: string(rune('A' + id))}).Error; err != nil {
			t.Fatal(err)
		}
	}
	links := []po.RoleMenu{
		{RoleID: 1, MenuID: 1},
		{RoleID: 1, MenuID: 2},
		{RoleID: 1, MenuID: 3, IsInherited: true},
		{RoleID: 2, MenuID: 1, IsInherited: true},
		{RoleID: 2, MenuID: 4},
	}
	if err := s.db.Create(&links).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

func readRoleMenuLinks(t *testing.T, s *suite) []po.RoleMenu {
	t.Helper()
	var links []po.RoleMenu
	if err := s.db.Order("role_id, menu_id").Find(&links).Error; err != nil {
		t.Fatal(err)
	}
	return links
}

func menuTransactionService(s *suite, manager tx.Manager, menus rbacdomain.MenuRepository, links rbacdomain.RoleMenuRepository) *app.MenuService {
	return app.NewMenuService(manager, s.roleRepo, menus, links, nil, nil, zap.NewNop())
}

func TestAssignRoleMenusInsertFailurePreservesPreviousLinks(t *testing.T) {
	s := newMenuTransactionSuite(t)
	before := readRoleMenuLinks(t, s)
	links := failingRoleMenuInsert{infra.NewRoleMenuRepository(s.db), errors.New("insert failed")}
	service := menuTransactionService(s, gormx.NewTxManager(s.db), infra.NewMenuRepository(s.db), links)
	if err := service.AssignRoleMenus(t.Context(), 1, []uint{2, 3, 4, 2}); err != rbacdomain.ErrUpdateFailed {
		t.Fatalf("error = %v, want %v", err, rbacdomain.ErrUpdateFailed)
	}
	if after := readRoleMenuLinks(t, s); !reflect.DeepEqual(after, before) {
		t.Fatalf("failed replacement changed links: got %+v, want %+v", after, before)
	}
}

func TestMenuDeleteFailurePreservesRoleLinks(t *testing.T) {
	s := newMenuTransactionSuite(t)
	before := readRoleMenuLinks(t, s)
	menus := failingMenuRemoval{infra.NewMenuRepository(s.db), errors.New("delete failed")}
	service := menuTransactionService(s, gormx.NewTxManager(s.db), menus, infra.NewRoleMenuRepository(s.db))
	if err := service.Delete(t.Context(), 1); err != rbacdomain.ErrDeleteFailed {
		t.Fatalf("error = %v, want %v", err, rbacdomain.ErrDeleteFailed)
	}
	if after := readRoleMenuLinks(t, s); !reflect.DeepEqual(after, before) {
		t.Fatalf("failed deletion changed links: got %+v, want %+v", after, before)
	}
	menu, err := menus.FindByID(t.Context(), 1)
	if err != nil || menu == nil {
		t.Fatalf("failed deletion removed menu: menu=%v error=%v", menu, err)
	}
}

type lockTrackingMenuRepository struct {
	rbacdomain.MenuRepository
	findForUpdateCalls int
	lockedID           uint
}

func (r *lockTrackingMenuRepository) FindByIDForUpdate(ctx context.Context, id uint) (*rbacdomain.Menu, error) {
	r.findForUpdateCalls++
	r.lockedID = id
	return r.MenuRepository.FindByIDForUpdate(ctx, id)
}

func TestMenuDeleteAcquiresRowLockBeforeCheckingChildren(t *testing.T) {
	s := newMenuTransactionSuite(t)
	tracker := &lockTrackingMenuRepository{MenuRepository: infra.NewMenuRepository(s.db)}
	service := menuTransactionService(s, gormx.NewTxManager(s.db), tracker, infra.NewRoleMenuRepository(s.db))

	if err := service.Delete(t.Context(), 1); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if tracker.findForUpdateCalls != 1 || tracker.lockedID != 1 {
		t.Fatalf("Delete did not acquire row lock on target menu: calls=%d, lockedID=%d, want 1 and 1",
			tracker.findForUpdateCalls, tracker.lockedID)
	}
}

func TestMenuDeletePreventsConcurrentChildCreation(t *testing.T) {
	s := newMenuTransactionSuite(t)
	manager := gormx.NewTxManager(s.db)
	repo := infra.NewMenuRepository(s.db)
	links := infra.NewRoleMenuRepository(s.db)
	service := menuTransactionService(s, manager, repo, links)

	if err := service.Delete(t.Context(), 1); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	parentID := uint(1)
	err := service.Create(t.Context(), app.CreateMenuParams{
		Title:    "concurrent-child",
		ParentID: &parentID,
	})
	if !errors.Is(err, rbacdomain.ErrParentMenuNotFound) {
		t.Fatalf("expected ErrParentMenuNotFound when creating child of deleted parent, got: %v", err)
	}
}

func TestMenuTransactionsCommitOnlyIntendedChanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		apply func(context.Context, *app.MenuService) error
		links []po.RoleMenu
		menus int64
	}{
		{
			name: "replace direct menus",
			apply: func(ctx context.Context, service *app.MenuService) error {
				return service.AssignRoleMenus(ctx, 1, []uint{2, 3, 4, 2})
			},
			links: []po.RoleMenu{{RoleID: 1, MenuID: 2}, {RoleID: 1, MenuID: 3, IsInherited: true}, {RoleID: 1, MenuID: 4}, {RoleID: 2, MenuID: 1, IsInherited: true}, {RoleID: 2, MenuID: 4}},
			menus: 4,
		},
		{
			name: "clear direct menus",
			apply: func(ctx context.Context, service *app.MenuService) error {
				return service.AssignRoleMenus(ctx, 1, nil)
			},
			links: []po.RoleMenu{{RoleID: 1, MenuID: 3, IsInherited: true}, {RoleID: 2, MenuID: 1, IsInherited: true}, {RoleID: 2, MenuID: 4}},
			menus: 4,
		},
		{
			name: "inherited selection clears only direct menus",
			apply: func(ctx context.Context, service *app.MenuService) error {
				return service.AssignRoleMenus(ctx, 1, []uint{3, 3})
			},
			links: []po.RoleMenu{{RoleID: 1, MenuID: 3, IsInherited: true}, {RoleID: 2, MenuID: 1, IsInherited: true}, {RoleID: 2, MenuID: 4}},
			menus: 4,
		},
		{
			name: "delete menu and all its links",
			apply: func(ctx context.Context, service *app.MenuService) error {
				return service.Delete(ctx, 1)
			},
			links: []po.RoleMenu{{RoleID: 1, MenuID: 2}, {RoleID: 1, MenuID: 3, IsInherited: true}, {RoleID: 2, MenuID: 4}},
			menus: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newMenuTransactionSuite(t)
			if err := tc.apply(t.Context(), s.menus); err != nil {
				t.Fatal(err)
			}
			assertMenuTransactionState(t, s, tc.links, tc.menus)
		})
	}
}

func TestMenuTransactionsRollBackAllWritesWithOuterTransaction(t *testing.T) {
	for _, operation := range []struct {
		name  string
		apply func(context.Context, *app.MenuService) error
	}{
		{"assign", func(ctx context.Context, service *app.MenuService) error {
			return service.AssignRoleMenus(ctx, 1, []uint{2, 4})
		}},
		{"delete", func(ctx context.Context, service *app.MenuService) error {
			return service.Delete(ctx, 1)
		}},
	} {
		for _, callerRollback := range []bool{false, true} {
			phase := "failure before commit"
			if callerRollback {
				phase = "caller rollback"
			}
			t.Run(operation.name+"/"+phase, func(t *testing.T) {
				s := newMenuTransactionSuite(t)
				before := readRoleMenuLinks(t, s)
				manager := gormx.NewTxManager(s.db)
				var err error
				if callerRollback {
					err = manager.Do(t.Context(), func(txCtx context.Context) error {
						if err := operation.apply(txCtx, s.menus); err != nil {
							return err
						}
						return injectedFailure
					})
				} else {
					service := menuTransactionService(s, failedCommit{base: manager}, infra.NewMenuRepository(s.db), infra.NewRoleMenuRepository(s.db))
					err = operation.apply(t.Context(), service)
				}
				if !errors.Is(err, injectedFailure) {
					t.Fatalf("error = %v, want transaction failure", err)
				}
				assertMenuTransactionState(t, s, before, 4)
			})
		}
	}
}

func assertMenuTransactionState(t *testing.T, s *suite, links []po.RoleMenu, menuCount int64) {
	t.Helper()
	if got := readRoleMenuLinks(t, s); !reflect.DeepEqual(got, links) {
		t.Fatalf("role links = %+v, want %+v", got, links)
	}
	var got int64
	if err := s.db.Model(&po.Menu{}).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != menuCount {
		t.Fatalf("menu count = %d, want %d", got, menuCount)
	}
}
