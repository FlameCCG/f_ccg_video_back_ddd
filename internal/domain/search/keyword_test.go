package search

import "testing"

func TestNormalizeHotKeywordCollapsesBlankAndTruncates(t *testing.T) {
	keywords := KeywordPolicy{}
	if got := keywords.NormalizeHotKeyword("  多  空格   词 "); got != "多 空格 词" {
		t.Fatalf("NormalizeHotKeyword() = %q", got)
	}

	if got := keywords.NormalizeHotKeyword("   "); got != "" {
		t.Fatalf("blank keyword should be dropped, got %q", got)
	}

	long := make([]rune, maxHotKeywordLength+10)
	for i := range long {
		long[i] = '词'
	}

	if got := keywords.NormalizeHotKeyword(string(long)); len([]rune(got)) != maxHotKeywordLength {
		t.Fatalf("truncated length = %d, want %d", len([]rune(got)), maxHotKeywordLength)
	}
}

func TestSelectMatchingTags(t *testing.T) {
	tags := []string{" 教程 ", "vlog", "Go 教程"}

	keywords := KeywordPolicy{}
	if got := keywords.SelectMatchingTags(tags, "教程"); len(got) != 2 || got[0] != "教程" || got[1] != "Go 教程" {
		t.Fatalf("literal matches = %v", got)
	}

	// 字面不含查询词时只回退一个标签，避免把整篇文档的标签灌进联想。
	if got := keywords.SelectMatchingTags(tags, "zzz"); len(got) != 1 || got[0] != "教程" {
		t.Fatalf("fallback matches = %v", got)
	}

	if got := keywords.SelectMatchingTags(nil, "x"); got != nil {
		t.Fatalf("empty tags should return nil, got %v", got)
	}
}

func TestMatchesUsername(t *testing.T) {
	keywords := KeywordPolicy{}
	if !keywords.MatchesUsername("Alice", "ali") {
		t.Fatal("case-insensitive match expected")
	}

	if keywords.MatchesUsername("Bob", "ali") {
		t.Fatal("unexpected match")
	}

	if !keywords.MatchesUsername("anyone", "  ") {
		t.Fatal("blank keyword should match everything")
	}
}

func TestSplitTags(t *testing.T) {
	got := (KeywordPolicy{}).SplitTags(" a , ,b ,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("SplitTags() = %v", got)
	}
}
