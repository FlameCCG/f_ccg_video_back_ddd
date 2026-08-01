package rbac

import (
	"context"
	"errors"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

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
			fallback: domain.ErrCreateFailed,
			call: func() error {
				return service.Create(context.Background(), CreateMenuParams{Title: "created"})
			},
		},
		{
			name:     "update",
			fallback: domain.ErrUpdateFailed,
			call: func() error {
				return service.Update(context.Background(), UpdateMenuParams{ID: 1, Title: &title})
			},
		},
		{
			name:     "move",
			fallback: domain.ErrSortFailed,
			call: func() error {
				return service.Move(context.Background(), 1, 2, MenuMoveBefore)
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
	if !errors.Is(err, domain.ErrReadMenuDataFailed) {
		t.Fatalf("error = %v, want public fallback %v", err, domain.ErrReadMenuDataFailed)
	}
}

type failingMenuTxManager struct {
	err error
}

func (m *failingMenuTxManager) Do(context.Context, func(context.Context) error) error {
	return m.err
}

type failingGenerationMenuRepository struct {
	domain.MenuRepository
	listErr error
}

func (r *failingGenerationMenuRepository) ListForGeneration(context.Context) ([]*domain.Menu, error) {
	return nil, r.listErr
}
