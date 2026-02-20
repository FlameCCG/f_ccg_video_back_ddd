package search

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxHotKeywordLength 热词最长保留的 rune 数，超出直接截断.
const maxHotKeywordLength = 50

// NormalizeKeyword 去掉关键词首尾空白（搜索入口统一入参形态）.
func NormalizeKeyword(keyword string) string {
	return strings.TrimSpace(keyword)
}

// LowerKeyword 去空白并转小写，用于命中 keyword 子字段与前缀查询.
func LowerKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

// EscapeESWildcard 转义 wildcard 特殊字符并丢弃控制字符；空串返回 false。
//
// 通配查询只在 keyword 子字段上做，避免为中间片段匹配新增 ngram 字段。
func EscapeESWildcard(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}

	var b strings.Builder
	b.Grow(len(s) + 4)

	for _, r := range s {
		switch r {
		case '\\', '*', '?', '[', ']', '{', '}', '(', ')', '"', ':', '/', '+', '-', '~', '!', '^':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if unicode.IsControl(r) {
				continue
			}

			b.WriteRune(r)
		}
	}

	out := b.String()
	if out == "" {
		return "", false
	}

	return out, true
}

// HighlightSuggestion 对 value 中与 prefix 字面匹配的首段做 <em> 包裹（大小写不敏感，按 rune）。
// 拼音命中但原文不含字面时无法高亮，保持原文。
func HighlightSuggestion(value, prefix string) string {
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

// NormalizeHotKeyword 压缩热词内部空白并截断到 50 个 rune；空白词返回空串（不入榜）。
func NormalizeHotKeyword(keyword string) string {
	keyword = strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
	if keyword == "" {
		return ""
	}

	if utf8.RuneCountInString(keyword) <= maxHotKeywordLength {
		return keyword
	}

	runes := []rune(keyword)

	return string(runes[:maxHotKeywordLength])
}

// SplitTags 把逗号分隔的标签串拆成去空白的标签列表.
func SplitTags(val string) []string {
	parts := strings.Split(val, ",")

	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}

// SelectMatchingTags 从视频文档的 tags 中挑出与查询词相关的标签（字面包含优先）。
//
// 字面命中优先返回全部命中项；拼音/分词命中但字面不含查询词时只回退 1 个标签，
// 避免把整篇文档的 tags 灌进联想结果。
func SelectMatchingTags(tags []string, prefix string) []string {
	if len(tags) == 0 {
		return nil
	}

	lower := LowerKeyword(prefix)
	matched := make([]string, 0, len(tags))
	rest := make([]string, 0, len(tags))

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}

		if lower != "" && strings.Contains(strings.ToLower(tag), lower) {
			matched = append(matched, tag)

			continue
		}

		rest = append(rest, tag)
	}

	if len(matched) > 0 {
		return matched
	}

	if len(rest) == 0 {
		return nil
	}

	return rest[:1]
}

// MatchesUsername 判断用户名是否本地字面命中关键词（大小写不敏感）；空关键词视为全部命中。
//
// 这是 @ 提及联想在 ES 不可用时的兜底匹配规则。
func MatchesUsername(username, keyword string) bool {
	keyword = LowerKeyword(keyword)
	if keyword == "" {
		return true
	}

	return strings.Contains(strings.ToLower(username), keyword)
}

// ParseDocumentID 解析 ES 文档 _id（形如 users:12 或 12）中的业务主键。
//
// 文档 _id 的「表名前缀 + 冒号 + 主键」格式由 River 同步侧写入，属于外部契约。
func ParseDocumentID(docID string) (uint, bool) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return 0, false
	}

	if parts := strings.SplitN(docID, ":", 2); len(parts) == 2 {
		docID = parts[1]
	}

	id, err := strconv.ParseUint(docID, 10, 64)
	if err != nil {
		return 0, false
	}

	return uint(id), true
}
