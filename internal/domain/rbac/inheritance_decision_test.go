package rbac_test

import (
	"errors"
	"testing"

	rbac "f_ccg_video_back_ddd/internal/domain/rbac"
)

type inheritanceFacts struct {
	direct, implicit       map[string][]string
	directErr, implicitErr map[string]error
}

func (f inheritanceFacts) DirectParents(role string) ([]string, error) {
	return f.direct[role], f.directErr[role]
}
func (f inheritanceFacts) ImplicitParents(role string) ([]string, error) {
	return f.implicit[role], f.implicitErr[role]
}

func TestInheritanceDecisionsClassifyPathsAndRejectCycles(t *testing.T) {
	p := rbac.Inherit{}
	facts := inheritanceFacts{
		direct:   map[string][]string{"child": {"middle", "other"}, "middle": {"root"}},
		implicit: map[string][]string{"child": {"middle", "other", "root"}, "middle": {"root"}},
	}
	for _, tc := range []struct {
		target string
		owned  bool
		kind   rbac.InheritKind
		top    string
	}{
		{"middle", true, rbac.InheritKindDirect, "middle"},
		{"root", true, rbac.InheritKindIndirect, "middle"},
		{"unrelated", false, rbac.InheritKindNone, ""},
	} {
		owned, kind, top, err := p.IsAlreadyInherited(facts, "child", tc.target)
		if err != nil || owned != tc.owned || kind != tc.kind || top != tc.top {
			t.Fatalf("target=%s: %v %v %s %v", tc.target, owned, kind, top, err)
		}
	}
	if cycle, err := p.WouldCreateCycle(facts, "root", "child"); err != nil || !cycle {
		t.Fatal("reverse reachability must form a cycle", err)
	}
	if cycle, err := p.WouldCreateCycle(facts, "child", "root"); err != nil || cycle {
		t.Fatal("existing forward inheritance is not a reverse cycle", err)
	}
	if top, kind, err := p.FindTopParentForOwner(facts, "child", "child"); err != nil || top != "" || kind != rbac.InheritKindNone {
		t.Fatal("own permission must not acquire an inheritance owner", err)
	}
}

func TestInheritanceDecisionsPreserveLookupFailureSemantics(t *testing.T) {
	failed := errors.New("relationship lookup failed")
	p := rbac.Inherit{}
	facts := inheritanceFacts{direct: map[string][]string{"child": {"root"}}, implicitErr: map[string]error{"child": failed}}
	if owned, kind, _, err := p.IsAlreadyInherited(facts, "child", "root"); err != nil || !owned || kind != rbac.InheritKindDirect {
		t.Fatal("direct ownership must not depend on unrelated implicit lookup", err)
	}
	if _, _, _, err := p.IsAlreadyInherited(facts, "child", "other"); !errors.Is(err, failed) {
		t.Fatal("implicit lookup failure was swallowed", err)
	}
	facts = inheritanceFacts{direct: map[string][]string{"child": {"middle"}}, implicit: map[string][]string{"child": {"root"}}, implicitErr: map[string]error{"middle": failed}}
	if owned, kind, top, err := p.IsAlreadyInherited(facts, "child", "root"); !owned || kind != rbac.InheritKindIndirect || top != "" || !errors.Is(err, failed) {
		t.Fatal("known indirect ownership must survive an owner explanation failure", owned, kind, top, err)
	}
}
