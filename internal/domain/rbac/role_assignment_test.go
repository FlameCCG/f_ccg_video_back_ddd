package rbac

import (
	"reflect"
	"testing"
)

func TestRoleAssignmentPreservesSetsAndRejectsDuplicate(t *testing.T) {
	one := RestoreRole(RoleSnapshot{ID: 1, Name: "one"})
	two := RestoreRole(RoleSnapshot{ID: 2, Name: "two"})
	three := RestoreRole(RoleSnapshot{ID: 3, Name: "three"})
	current := []*Role{one, two}
	policy := RoleAssignmentPolicy{}
	if _, err := policy.Plan("assign", current, []*Role{two}, 1); err == nil {
		t.Fatal("duplicate direct assignment accepted")
	}
	if !reflect.DeepEqual(current, []*Role{one, two}) {
		t.Fatal("failed assignment changed the input set")
	}
	if _, err := policy.Plan("replace", current, []*Role{three}, 2); err != ErrSomeRolesNotFound {
		t.Fatalf("missing role error changed: %v", err)
	}
	for _, tc := range []struct {
		mode           string
		selected, want []*Role
	}{
		{"assign", []*Role{three}, []*Role{one, two, three}},
		{"remove", []*Role{one}, []*Role{two}},
		{"replace", []*Role{three}, []*Role{three}},
	} {
		got, err := policy.Plan(tc.mode, current, tc.selected, len(tc.selected))
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s got=%v err=%v", tc.mode, got, err)
		}
	}
	if policy.EnsureMenuSelection(1, 2) != ErrSomeMenusNotFound || one.EnsureInitialMenus(1, 2) != ErrPartialMenusNotFound {
		t.Fatal("menu selection error contracts changed")
	}
}
