package rbac_test

import (
	"errors"
	"reflect"
	"testing"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

func TestMenuMovePlansAndImmutableFacts(t *testing.T) {
	for _, tt := range []struct {
		moving, target uint
		position       string
		want           []uint
	}{
		{3, 1, rbacdomain.MenuMoveBefore, []uint{3, 1, 2}},
		{1, 3, rbacdomain.MenuMoveAfter, []uint{2, 3, 1}},
		{1, 2, rbacdomain.MenuMoveBefore, []uint{1, 2, 3}},
		{3, 2, rbacdomain.MenuMoveAfter, []uint{1, 2, 3}},
	} {
		facts := []uint{1, 2, 3}
		moving, target := rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: tt.moving}), rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: tt.target})
		got, err := (rbacdomain.MenuOrder{}).PlanMove(moving, target, facts, tt.position)
		if err != nil || !reflect.DeepEqual(got, tt.want) || !reflect.DeepEqual(facts, []uint{1, 2, 3}) {
			t.Fatalf("plan=%v facts=%v err=%v", got, facts, err)
		}
	}
	parent := uint(8)
	if _, err := (rbacdomain.MenuOrder{}).PlanMove(rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 1}), rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2, ParentID: &parent}), []uint{1, 2}, rbacdomain.MenuMoveAfter); !errors.Is(err, rbacdomain.ErrMenuMoveAcrossLevels) {
		t.Fatal(err)
	}
}

func TestMenuMoveEligibilityPreservesErrorPriority(t *testing.T) {
	parent := uint(8)
	zero := uint(0)
	rootMenu := rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 1})
	childMenu := rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2, ParentID: &parent})
	zeroParentMenu := rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 3, ParentID: &zero})
	for _, tc := range []struct {
		name           string
		moving, target *rbacdomain.Menu
		position       string
		want           error
	}{
		{"missing moving", nil, childMenu, "invalid", rbacdomain.ErrMenuNotFound},
		{"missing target", rootMenu, nil, "invalid", rbacdomain.ErrMenuNotFound},
		{"invalid position before level", rootMenu, childMenu, "invalid", rbacdomain.ErrMenuMovePosition},
		{"different levels", rootMenu, childMenu, rbacdomain.MenuMoveBefore, rbacdomain.ErrMenuMoveAcrossLevels},
		{"equivalent roots", rootMenu, zeroParentMenu, rbacdomain.MenuMoveAfter, nil},
		{"same parent", childMenu, childMenu, rbacdomain.MenuMoveBefore, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := (rbacdomain.MenuOrder{}).EnsureMoveAllowed(tc.moving, tc.target, tc.position); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v, want %v", err, tc.want)
			}
		})
	}
}

func TestMenuInsertionAndPlacementPlans(t *testing.T) {
	for _, tt := range []struct {
		requested uint
		want      []uint
		order     uint
	}{{1, []uint{4, 1, 2, 3}, 1}, {0, []uint{1, 2, 3, 4}, 4}, {99, []uint{1, 2, 3, 4}, 4}} {
		plan := (rbacdomain.MenuOrder{}).PlanInsertion(4, tt.requested, []uint{1, 2, 3})
		if plan.SortOrder != tt.order || !reflect.DeepEqual(plan.Sequence, tt.want) {
			t.Fatal(plan)
		}
	}
	plan := (rbacdomain.MenuOrder{}).PlanPlacement(rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2}), nil, nil, []uint{1, 2, 3}, nil)
	if plan.ParentChanged || !reflect.DeepEqual(plan.DestinationOrder, []uint{1, 2, 3}) {
		t.Fatal(plan)
	}
	parent := uint(9)
	plan = (rbacdomain.MenuOrder{}).PlanPlacement(rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2}), &parent, nil, []uint{1, 2, 3}, []uint{4})
	if !plan.ParentChanged || !reflect.DeepEqual(plan.SourceOrder, []uint{1, 3}) || !reflect.DeepEqual(plan.DestinationOrder, []uint{4, 2}) {
		t.Fatal(plan)
	}
}
