package rbac

import (
	"errors"
	"reflect"
	"testing"
)

func TestMenuSelfMovePreservesSequenceAndErrorPriority(t *testing.T) {
	menu := RestoreMenu(MenuSnapshot{ID: 2})
	sequence := []uint{1, 2, 3}
	got, err := (MenuOrder{}).PlanMove(menu, menu, sequence, "invalid")
	if err != nil || !reflect.DeepEqual(got, sequence) {
		t.Fatalf("self move must be a no-op before position validation: %v, %v", got, err)
	}
	got[0] = 9
	if sequence[0] != 1 {
		t.Fatal("no-op result aliases input")
	}
	if title, err := (MenuSuggestion{}).GenerationTitle(" 菜单 "); title != "菜单" || err != nil {
		t.Fatalf("title normalization: %q, %v", title, err)
	}
	if _, err := (MenuSuggestion{}).GenerationTitle("\t "); !errors.Is(err, ErrMenuGenerationTitleEmpty) {
		t.Fatal(err)
	}
}
