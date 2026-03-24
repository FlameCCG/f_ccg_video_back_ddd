package ai

import (
	"reflect"
	"testing"
)

func TestAnalysisHistoryCutsBeforeFilteringAndCopiesImages(t *testing.T) {
	policy := HistoryPolicy{}
	facts := []HistoryMessage{{Present: true, User: true}, {Present: true}, {}, {Present: true, System: true}, {Present: true, User: true}}
	if got := policy.AnalysisPositions(facts, 3); !reflect.DeepEqual(got, []int{4}) {
		t.Fatalf("analysis window changed: %v", got)
	}
	if got := policy.AnalysisPositions(facts, 0); !reflect.DeepEqual(got, []int{0, 1, 4}) {
		t.Fatalf("unbounded history changed: %v", got)
	}
	images := []string{"first", "second", "third", "fourth"}
	selected := policy.QueryImages(images)
	if !reflect.DeepEqual(selected, images[:3]) {
		t.Fatalf("image priority/limit changed: %v", selected)
	}
	selected[0] = "changed"
	if images[0] != "first" || policy.QueryImages(nil) != nil || policy.QueryImages([]string{}) == nil {
		t.Fatal("image selection aliases input or changes empty representation")
	}
}

func TestThinkingPolicyKeepsGateDefaultsAndValidation(t *testing.T) {
	on, off := true, false
	options := Options{{Value: "high"}, {Value: "max"}}
	for _, tc := range []struct {
		name       string
		configured bool
		requested  *bool
		effort     string
		want       ThinkingSelection
		invalid    bool
	}{
		{"site disabled", false, &on, "invalid", ThinkingSelection{}, false},
		{"request disabled", true, &off, "invalid", ThinkingSelection{}, false},
		{"default enabled", true, nil, "", ThinkingSelection{true, "high"}, false},
		{"legacy effort", true, &on, "xhigh", ThinkingSelection{true, "max"}, false},
		{"invalid effort", true, &on, "invalid", ThinkingSelection{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := options.ResolveThinking(tc.configured, tc.requested, tc.effort, "")
			if (err != nil) != tc.invalid || !tc.invalid && got != tc.want {
				t.Fatalf("got %+v, %v; want %+v, invalid=%v", got, err, tc.want, tc.invalid)
			}
		})
	}
	if !on || off {
		t.Fatal("selection mutated request pointers")
	}
}
