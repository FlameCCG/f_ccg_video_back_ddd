package search

import "testing"

func TestHighlightSuggestionMidStringAndChinese(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		prefix string
		want   string
	}{
		{
			name:   "mid ascii segment",
			value:  "毛民_ce6ffa102500",
			prefix: "ce6",
			want:   "毛民_<em>ce6</em>ffa102500",
		},
		{
			name:   "single chinese",
			value:  "毛民_ce6ffa102500",
			prefix: "毛",
			want:   "<em>毛</em>民_ce6ffa102500",
		},
		{
			name:   "case insensitive",
			value:  "HelloWorld",
			prefix: "hell",
			want:   "<em>Hell</em>oWorld",
		},
		{
			name:   "pinyin no literal match keeps original",
			value:  "毛民_ce6ffa102500",
			prefix: "mao",
			want:   "毛民_ce6ffa102500",
		},
		{
			name:   "empty prefix",
			value:  "abc",
			prefix: "",
			want:   "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HighlightSuggestion(tt.value, tt.prefix); got != tt.want {
				t.Fatalf("HighlightSuggestion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeESWildcard(t *testing.T) {
	got, ok := EscapeESWildcard("ce6*")
	if !ok {
		t.Fatal("expected ok")
	}

	if got != `ce6\*` {
		t.Fatalf("got %q", got)
	}

	if _, ok := EscapeESWildcard("   "); ok {
		t.Fatal("blank should be rejected")
	}
}

func TestParseDocumentID(t *testing.T) {
	tests := []struct {
		name  string
		docID string
		want  uint
		ok    bool
	}{
		{name: "with prefix", docID: "users:12", want: 12, ok: true},
		{name: "plain id", docID: "34", want: 34, ok: true},
		{name: "invalid", docID: "users:abc", want: 0, ok: false},
		{name: "empty", docID: "", want: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseDocumentID(tt.docID)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ParseDocumentID(%q) = (%d, %v), want (%d, %v)", tt.docID, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNormalizeHotKeywordCollapsesBlankAndTruncates(t *testing.T) {
	if got := NormalizeHotKeyword("  多  空格   词 "); got != "多 空格 词" {
		t.Fatalf("NormalizeHotKeyword() = %q", got)
	}

	if got := NormalizeHotKeyword("   "); got != "" {
		t.Fatalf("blank keyword should be dropped, got %q", got)
	}

	long := make([]rune, maxHotKeywordLength+10)
	for i := range long {
		long[i] = '词'
	}

	if got := NormalizeHotKeyword(string(long)); len([]rune(got)) != maxHotKeywordLength {
		t.Fatalf("truncated length = %d, want %d", len([]rune(got)), maxHotKeywordLength)
	}
}

func TestSelectMatchingTags(t *testing.T) {
	tags := []string{" 教程 ", "vlog", "Go 教程"}

	if got := SelectMatchingTags(tags, "教程"); len(got) != 2 || got[0] != "教程" || got[1] != "Go 教程" {
		t.Fatalf("literal matches = %v", got)
	}

	// 字面不含查询词时只回退一个标签，避免把整篇文档的标签灌进联想。
	if got := SelectMatchingTags(tags, "zzz"); len(got) != 1 || got[0] != "教程" {
		t.Fatalf("fallback matches = %v", got)
	}

	if got := SelectMatchingTags(nil, "x"); got != nil {
		t.Fatalf("empty tags should return nil, got %v", got)
	}
}

func TestMatchesUsername(t *testing.T) {
	if !MatchesUsername("Alice", "ali") {
		t.Fatal("case-insensitive match expected")
	}

	if MatchesUsername("Bob", "ali") {
		t.Fatal("unexpected match")
	}

	if !MatchesUsername("anyone", "  ") {
		t.Fatal("blank keyword should match everything")
	}
}

func TestSplitTags(t *testing.T) {
	got := SplitTags(" a , ,b ,")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("SplitTags() = %v", got)
	}
}
