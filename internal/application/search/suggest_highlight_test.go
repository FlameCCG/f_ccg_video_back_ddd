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
			if got := (SuggestionHighlighter{}).HighlightSuggestion(tt.value, tt.prefix); got != tt.want {
				t.Fatalf("HighlightSuggestion() = %q, want %q", got, tt.want)
			}
		})
	}
}
