package service_test

import (
	"errors"
	"reflect"
	"testing"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	"f_ccg_video_back_ddd/internal/domain/rbac/entity"
	"f_ccg_video_back_ddd/internal/domain/rbac/service"
)

func TestMenuMovePlansAndImmutableFacts(t *testing.T) {
	for _, tt := range []struct {
		moving, target uint
		position       string
		want           []uint
	}{
		{3, 1, service.MenuMoveBefore, []uint{3, 1, 2}},
		{1, 3, service.MenuMoveAfter, []uint{2, 3, 1}},
		{1, 2, service.MenuMoveBefore, []uint{1, 2, 3}},
		{3, 2, service.MenuMoveAfter, []uint{1, 2, 3}},
	} {
		facts := []uint{1, 2, 3}
		moving, target := &entity.Menu{ID: tt.moving}, &entity.Menu{ID: tt.target}
		got, err := service.PlanMenuMove(moving, target, facts, tt.position)
		if err != nil || !reflect.DeepEqual(got, tt.want) || !reflect.DeepEqual(facts, []uint{1, 2, 3}) {
			t.Fatalf("plan=%v facts=%v err=%v", got, facts, err)
		}
	}
	parent := uint(8)
	if _, err := service.PlanMenuMove(&entity.Menu{ID: 1}, &entity.Menu{ID: 2, ParentID: &parent}, []uint{1, 2}, service.MenuMoveAfter); !errors.Is(err, rbacerrors.ErrMenuMoveAcrossLevels) {
		t.Fatal(err)
	}
}

func TestMenuInsertionAndPlacementPlans(t *testing.T) {
	for _, tt := range []struct {
		requested uint
		want      []uint
		order     uint
	}{{1, []uint{4, 1, 2, 3}, 1}, {0, []uint{1, 2, 3, 4}, 4}, {99, []uint{1, 2, 3, 4}, 4}} {
		plan := service.PlanMenuInsertion(4, tt.requested, []uint{1, 2, 3})
		if plan.SortOrder != tt.order || !reflect.DeepEqual(plan.Sequence, tt.want) {
			t.Fatal(plan)
		}
	}
	plan := service.PlanMenuPlacement(&entity.Menu{ID: 2}, nil, nil, []uint{1, 2, 3}, nil)
	if plan.ParentChanged || !reflect.DeepEqual(plan.DestinationOrder, []uint{1, 2, 3}) {
		t.Fatal(plan)
	}
	parent := uint(9)
	plan = service.PlanMenuPlacement(&entity.Menu{ID: 2}, &parent, nil, []uint{1, 2, 3}, []uint{4})
	if !plan.ParentChanged || !reflect.DeepEqual(plan.SourceOrder, []uint{1, 3}) || !reflect.DeepEqual(plan.DestinationOrder, []uint{4, 2}) {
		t.Fatal(plan)
	}
}
