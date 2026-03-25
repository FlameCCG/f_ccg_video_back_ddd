package search

import (
	"reflect"
	"strings"
	"testing"
)

func TestHotKeywordSelectionKeepsFirstNormalizedCandidate(t *testing.T) {
	got := (KeywordPolicy{}).SelectHotKeywords([]string{" ", " Go\tVideo ", "Go Video", "go video", strings.Repeat("字", 51), strings.Repeat("字", 50)})
	want := []HotKeywordSelection{{1, "Go Video"}, {3, "go video"}, {4, strings.Repeat("字", 50)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("first-hit/case/rune semantics changed: %v", got)
	}
}
