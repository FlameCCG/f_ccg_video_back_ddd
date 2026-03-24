package rbac

import "testing"

func TestMenuGrantRequiresIdentifiedCreatorAndExistingCurrentRole(t *testing.T) {
	policy := MenuGrantPolicy{}
	if policy.EnsureCreator(0) != ErrMenuCreatorRoleMissing || policy.EnsureCreator(7) != nil {
		t.Fatal("creator eligibility changed")
	}
	for _, tc := range []struct {
		found   bool
		roleID  uint
		allowed bool
	}{{false, 0, false}, {false, 9, false}, {true, 0, false}, {true, 9, true}} {
		err := policy.EnsureCurrentRole(tc.found, tc.roleID)
		if tc.allowed && err != nil || !tc.allowed && err != ErrMenuCreatorRoleMissing {
			t.Fatalf("found=%v role=%d error=%v", tc.found, tc.roleID, err)
		}
	}
}
