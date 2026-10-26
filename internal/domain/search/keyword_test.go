package search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testKeyword = "毛民_ce6ffa102500"
)

func TestHighlightSuggestionMidStringAndChinese(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		prefix string
		want   string
	}{
		{
			name:   "mid ascii segment",
			value:  testKeyword,
			prefix: "ce6",
			want:   "毛民_<em>ce6</em>ffa102500",
		},
		{
			name:   "single chinese",
			value:  testKeyword,
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
			value:  testKeyword,
			prefix: "mao",
			want:   testKeyword,
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
			t.Parallel()

			{
				got := HighlightSuggestion(tt.value, tt.prefix)
				require.Equal(t, tt.want, got, "HighlightSuggestion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeESWildcard(t *testing.T) {
	t.Parallel()

	got, ok := EscapeESWildcard("ce6*")
	require.True(t, ok, "expected ok")

	require.Equal(t, `ce6\*`, got, "got %q", got)

	{
		_, ok := EscapeESWildcard("   ")
		require.False(t, ok, "blank should be rejected")
	}
}

func TestParseDocumentID(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			got, ok := ParseDocumentID(tt.docID)
			require.Equal(t, tt.want, got, "ParseDocumentID(%q) = (%d, %v), want (%d, %v)", tt.docID, got, ok, tt.want, tt.ok)
			require.Equal(t, ok, tt.ok, "ParseDocumentID(%q) = (%d, %v), want (%d, %v)", tt.docID, got, ok, tt.want, tt.ok)
		})
	}
}

func TestNormalizeHotKeywordCollapsesBlankAndTruncates(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeHotKeyword("  多  空格   词 ")
		require.Equal(t, "多 空格 词", got, "NormalizeHotKeyword() = %q", got)
	}

	{
		got := NormalizeHotKeyword("   ")
		require.Empty(t, got, "blank keyword should be dropped, got %q", got)
	}

	long := make([]rune, maxHotKeywordLength+10)
	for i := range long {
		long[i] = '词'
	}

	{
		got := NormalizeHotKeyword(string(long))
		require.Len(t, []rune(got), maxHotKeywordLength, "truncated length = %d, want %d", len([]rune(got)), maxHotKeywordLength)
	}
}

func TestSelectMatchingTags(t *testing.T) {
	t.Parallel()

	tags := []string{" 教程 ", "vlog", "Go 教程"}

	{
		got := SelectMatchingTags(tags, "教程")
		require.Len(t, got, 2, "literal matches = %v", got)
		require.Equal(t, "教程", got[0], "literal matches = %v", got)
		require.Equal(t, "Go 教程", got[1], "literal matches = %v", got)
	}

	// 字面不含查询词时只回退一个标签，避免把整篇文档的标签灌进联想。
	{
		got := SelectMatchingTags(tags, "zzz")
		require.Len(t, got, 1, "fallback matches = %v", got)
		require.Equal(t, "教程", got[0], "fallback matches = %v", got)
	}

	{
		got := SelectMatchingTags(nil, "x")
		require.Nil(t, got, "empty tags should return nil, got %v", got)
	}
}

func TestMatchesUsername(t *testing.T) {
	t.Parallel()

	require.True(t, MatchesUsername("Alice", "ali"), "case-insensitive match expected")

	require.False(t, MatchesUsername("Bob", "ali"), "unexpected match")

	require.True(t, MatchesUsername("anyone", "  "), "blank keyword should match everything")
}

func TestSplitTags(t *testing.T) {
	t.Parallel()

	got := SplitTags(" a , ,b ,")
	require.Len(t, got, 2, "SplitTags() = %v", got)
	require.Equal(t, "a", got[0], "SplitTags() = %v", got)
	require.Equal(t, "b", got[1], "SplitTags() = %v", got)
}
