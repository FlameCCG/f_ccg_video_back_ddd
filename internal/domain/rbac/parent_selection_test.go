package rbac_test

import (
	"reflect"
	"slices"
	"testing"

	rbac "f_ccg_video_back_ddd/internal/domain/rbac"
)

func TestParentSelectionKeepsOrderAndExcludesOwnDirectAndDuplicateAncestors(t *testing.T) {
	direct := []string{" middle ", "", "other", "middle"}
	implicit := []string{"child", "root", " middle ", "root", "", "old", "other"}
	directBefore, implicitBefore := slices.Clone(direct), slices.Clone(implicit)
	selected := (rbac.Inherit{}).SelectParents("child", direct, implicit)
	if !reflect.DeepEqual(selected.Direct, []string{"middle", "other"}) || !reflect.DeepEqual(selected.Indirect, []string{"root", "old"}) {
		t.Fatalf("incorrect inheritance selection: %+v", selected)
	}
	selected.Direct[0], selected.Indirect[0] = "changed", "changed"
	if !slices.Equal(direct, directBefore) || !slices.Equal(implicit, implicitBefore) {
		t.Fatal("parent selection exposed or mutated input slices")
	}
	empty := (rbac.Inherit{}).SelectParents("child", nil, nil)
	if empty.Direct == nil || empty.Indirect == nil || len(empty.Direct)+len(empty.Indirect) != 0 {
		t.Fatalf("empty inheritance lists must remain arrays: %+v", empty)
	}
}
