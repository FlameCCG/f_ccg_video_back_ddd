package rbac

import (
	"context"
	"errors"
	"testing"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

func TestMenuMutationsPreserveUnknownTransactionErrorForHTTPBoundary(t *testing.T) {
	t.Parallel()

	cause := errors.New("commit failed")
	service := NewMenuService(
		&failingMenuTxManager{err: cause},
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)
	title := "updated"

	tests := []struct {
		name     string
		fallback error
		call     func() error
	}{
		{
			name:     "create",
			fallback: rbacdomain.ErrCreateFailed,
			call: func() error {
				return service.Create(context.Background(), CreateMenuParams{Title: "created"})
			},
		},
		{
			name:     "update",
			fallback: rbacdomain.ErrUpdateFailed,
			call: func() error {
				return service.Update(context.Background(), UpdateMenuParams{ID: 1, Title: &title})
			},
		},
		{
			name:     "move",
			fallback: rbacdomain.ErrSortFailed,
			call: func() error {
				return service.Move(context.Background(), 1, 2, MenuMoveBefore)
			},
		},
		{
			name:     "delete",
			fallback: rbacdomain.ErrDeleteFailed,
			call: func() error {
				return service.Delete(context.Background(), 1)
			},
		},
		{
			name:     "assign",
			fallback: rbacdomain.ErrUpdateFailed,
			call: func() error {
				return service.AssignRoleMenus(context.Background(), 1, []uint{2})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.call()
			if !errors.Is(err, cause) {
				t.Fatalf("error = %v, want underlying cause %v", err, cause)
			}
			if errors.Is(err, tt.fallback) {
				t.Fatalf("application mapped error to HTTP fallback too early: %v", err)
			}
		})
	}
}

func TestMenuGenerationPreservesInfrastructureCauseAndPublicFallback(t *testing.T) {
	t.Parallel()

	cause := errors.New("query failed")
	service := NewMenuService(
		nil,
		nil,
		&failingGenerationMenuRepository{listErr: cause},
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	_, err := service.Generate(context.Background(), "菜单", nil)
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want underlying cause %v", err, cause)
	}
	if !errors.Is(err, rbacdomain.ErrReadMenuDataFailed) {
		t.Fatalf("error = %v, want public fallback %v", err, rbacdomain.ErrReadMenuDataFailed)
	}
}

type failingMenuTxManager struct {
	err error
}

func (m *failingMenuTxManager) Do(context.Context, func(context.Context) error) error {
	return m.err
}

type failingGenerationMenuRepository struct {
	rbacdomain.MenuRepository
	listErr error
}

func (r *failingGenerationMenuRepository) ListForGeneration(context.Context) ([]*rbacdomain.Menu, error) {
	return nil, r.listErr
}

func TestMenuMoveChecksLevelBeforeReadingSiblings(t *testing.T) {
	parent := uint(7)
	root := uint(0)
	for _, tc := range []struct {
		name         string
		movingParent *uint
		targetParent *uint
		acrossLevels bool
	}{
		{"root to child", nil, &parent, true},
		{"child to root", &parent, nil, true},
		{"same parent", &parent, &parent, false},
		{"nil and zero roots", nil, &root, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := errors.New("sibling query unavailable")
			repo := &moveLevelMenuRepository{
				moving:  rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 1, ParentID: tc.movingParent}),
				target:  rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2, ParentID: tc.targetParent}),
				listErr: cause,
			}
			service := NewMenuService(moveLevelTxManager{}, nil, repo, nil, nil, nil, zap.NewNop())
			err := service.Move(t.Context(), 1, 2, MenuMoveBefore)
			if tc.acrossLevels {
				if !errors.Is(err, rbacdomain.ErrMenuMoveAcrossLevels) || repo.siblingReads != 0 {
					t.Fatalf("err=%v sibling reads=%d; want cross-level error before sibling query", err, repo.siblingReads)
				}
			} else if !errors.Is(err, cause) || repo.siblingReads != 1 {
				t.Fatalf("err=%v sibling reads=%d; want original sibling query error", err, repo.siblingReads)
			}
		})
	}
}

type moveLevelTxManager struct{}

func (moveLevelTxManager) Do(ctx context.Context, action func(context.Context) error) error {
	return action(ctx)
}

type moveLevelMenuRepository struct {
	rbacdomain.MenuRepository
	moving, target *rbacdomain.Menu
	listErr        error
	siblingReads   int
}

func (r *moveLevelMenuRepository) FindByIDsForUpdate(context.Context, []uint) ([]*rbacdomain.Menu, error) {
	return []*rbacdomain.Menu{r.moving, r.target}, nil
}

func (r *moveLevelMenuRepository) ListSiblingIDsForUpdate(context.Context, *uint) ([]uint, error) {
	r.siblingReads++
	return nil, r.listErr
}
