package search

import (
	"strings"
	"unicode/utf8"
)

// maxHotKeywordLength 热词最长保留的 rune 数，超出直接截断.
const maxHotKeywordLength = 50

// KeywordPolicy 是搜索关键词的规范化与字面命中规则。
//
// 零值即可使用。通配转义、文档 _id、缓存键和索引名不属于本策略。
type KeywordPolicy struct{}

// NormalizeKeyword 去掉关键词首尾空白（搜索入口统一入参形态）.
func (KeywordPolicy) NormalizeKeyword(keyword string) string {
	return strings.TrimSpace(keyword)
}

// LowerKeyword 去空白并转小写，用于命中 keyword 子字段与前缀查询.
func (KeywordPolicy) LowerKeyword(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

// NormalizeHotKeyword 压缩热词内部空白并截断到 50 个 rune；空白词返回空串（不入榜）。
func (KeywordPolicy) NormalizeHotKeyword(keyword string) string {
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
func (KeywordPolicy) SplitTags(val string) []string {
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
func (p KeywordPolicy) SelectMatchingTags(tags []string, prefix string) []string {
	if len(tags) == 0 {
		return nil
	}

	lower := p.LowerKeyword(prefix)
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
func (p KeywordPolicy) MatchesUsername(username, keyword string) bool {
	keyword = p.LowerKeyword(keyword)
	if keyword == "" {
		return true
	}

	return strings.Contains(strings.ToLower(username), keyword)
}
