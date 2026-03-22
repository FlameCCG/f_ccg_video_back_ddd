package search

import (
	"strings"
	"unicode"
)

// SuggestionHighlighter renders the existing suggestion markup.
type SuggestionHighlighter struct{}

// HighlightSuggestion 对 value 中与 prefix 字面匹配的首段做 <em> 包裹（大小写不敏感，按 rune）。
// 拼音命中但原文不含字面时无法高亮，保持原文。
func (SuggestionHighlighter) HighlightSuggestion(value, prefix string) string {
	if prefix == "" || value == "" {
		return value
	}

	valueRunes := []rune(value)
	lowerValueRunes := make([]rune, len(valueRunes))

	for i, r := range valueRunes {
		lowerValueRunes[i] = unicode.ToLower(r)
	}

	prefixRunes := []rune(strings.ToLower(prefix))
	if len(prefixRunes) == 0 || len(prefixRunes) > len(lowerValueRunes) {
		return value
	}

	start := -1

	for i := 0; i <= len(lowerValueRunes)-len(prefixRunes); i++ {
		match := true

		for j := range prefixRunes {
			if lowerValueRunes[i+j] != prefixRunes[j] {
				match = false

				break
			}
		}

		if match {
			start = i

			break
		}
	}

	if start < 0 {
		return value
	}

	end := start + len(prefixRunes)

	return string(valueRunes[:start]) + "<em>" + string(valueRunes[start:end]) + "</em>" + string(valueRunes[end:])
}
