package rbac_test

import (
	"context"
	"errors"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	"f_ccg_video_back_ddd/internal/domain/rbac/entity"
	"f_ccg_video_back_ddd/internal/domain/rbac/repository"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"

	"go.uber.org/zap"
)

type failingMenuSort struct {
	repository.MenuRepository
	calls   int
	failure error
}

func (r *failingMenuSort) UpdateSortOrder(ctx context.Context, id, order uint) error {
	if err := r.MenuRepository.UpdateSortOrder(ctx, id, order); err != nil {
		return err
	}
	r.calls++
	if r.calls == 2 {
		return r.failure
	}
	return nil
}

func TestMenuSortSaveFailureRollsBackSequence(t *testing.T) {
	s := newSuite(t)
	repo := infra.NewMenuRepository(s.db)
	for i := uint(1); i <= 3; i++ {
		if err := repo.Create(t.Context(), &entity.Menu{ID: i, Title: string(rune('A' + i)), SortOrder: i}); err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("sort failed")
	wrapped := &failingMenuSort{MenuRepository: repo, failure: failure}
	service := app.NewMenuService(gormx.NewTxManager(s.db), nil, wrapped, nil, nil, nil, zap.NewNop())
	if err := service.Move(t.Context(), 3, 1, app.MenuMoveBefore); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	menus, err := repo.ListOrdered(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range menus {
		if m.ID != uint(i+1) || m.SortOrder != uint(i+1) {
			t.Fatalf("partial reorder: %+v", menus)
		}
	}
}
