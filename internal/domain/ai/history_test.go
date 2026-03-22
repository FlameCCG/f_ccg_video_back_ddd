package ai

import (
	"reflect"
	"testing"
)

func TestHistorySelectionPreservesSystemsAndNewestOverBudgetMessage(t *testing.T) {
	facts := []HistoryMessage{
		{Present: true, System: true, TextRunes: 100},
		{Present: true, User: true, TextRunes: 3},
		{Present: true, TextRunes: 3},
		{},
		{Present: true, User: true, TextRunes: 20},
	}
	p := HistoryPolicy{}
	if got := p.Select(facts, 20, 5); !reflect.DeepEqual(got, []int{0, 4}) {
		t.Fatalf("newest over-budget user lost: %v", got)
	}
	if got := p.Select(facts, 2, 100); !reflect.DeepEqual(got, []int{0, 4}) {
		t.Fatalf("history must start with a user: %v", got)
	}
	if got := p.Select(facts, 0, 0); !reflect.DeepEqual(got, []int{0, 1, 2, 4}) {
		t.Fatalf("default history changed: %v", got)
	}
	if turns, chars := p.Limits(0, 0); turns != 20 || chars != 24000 {
		t.Fatalf("history defaults changed: %d %d", turns, chars)
	}
}
