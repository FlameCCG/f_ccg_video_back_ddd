package search

import (
	"reflect"
	"testing"
)

func TestSearchSelectionPoliciesPreserveOrderingAndDistinctness(t *testing.T) {
	order := MentionOrder{}
	if !order.Less(MentionCandidate{ID: 9, IsFriend: true}, MentionCandidate{ID: 1, FollowerCount: 100}) ||
		!order.Less(MentionCandidate{ID: 9, FollowerCount: 2}, MentionCandidate{ID: 1, FollowerCount: 1}) ||
		!order.Less(MentionCandidate{ID: 1}, MentionCandidate{ID: 2}) {
		t.Fatal("mention precedence changed")
	}
	groups := [][]SuggestionCandidate{{{Kind: "video", Value: "Go"}, {Kind: "video", Value: "go"}}, {{Kind: "user", Value: "Go"}}}
	if got := (SuggestionPolicy{}).Select(3, true, groups...); !reflect.DeepEqual(got, []SuggestionSelection{{0, 0}, {1, 0}}) {
		t.Fatal(got)
	}
	if got := (SuggestionPolicy{}).Select(3, false, groups...); !reflect.DeepEqual(got, []SuggestionSelection{{0, 0}, {0, 1}, {1, 0}}) {
		t.Fatal(got)
	}
	if got := (SuggestionPolicy{}).Select(1, true, groups...); !reflect.DeepEqual(got, []SuggestionSelection{{0, 0}}) {
		t.Fatal(got)
	}
	if got := (RelatedTagPolicy{}).Select([]string{" Go ", "", "Go", "go"}); !reflect.DeepEqual(got, []string{"Go", "go"}) {
		t.Fatal(got)
	}
	tags := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	if got := (RelatedTagPolicy{}).Select(tags); len(got) != 10 || got[9] != "j" {
		t.Fatal(got)
	}
}

func TestRankFusionPreservesTiesDuplicatesAndUnboundedQueries(t *testing.T) {
	policy := RankFusion{}
	if got := policy.FuseRRF(0, 3, []uint{1, 2}, []uint{2, 3}); !reflect.DeepEqual(got, []uint{2, 1, 3}) {
		t.Fatal(got)
	}
	if got := policy.FuseRRF(60, 0, []uint{5}, []uint{7}); !reflect.DeepEqual(got, []uint{5, 7}) {
		t.Fatal(got)
	}
	if got := policy.FuseRRF(60, 1, []uint{1, 2, 2}); !reflect.DeepEqual(got, []uint{2}) {
		t.Fatal(got)
	}
	if got := policy.TopIDs([]uint{0, 3, 3, 2}, 0); !reflect.DeepEqual(got, []uint{0, 3, 2}) {
		t.Fatal(got)
	}
}
