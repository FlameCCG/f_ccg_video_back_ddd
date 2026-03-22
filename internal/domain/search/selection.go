package search

import "strings"

// MentionCandidate 是提及排序需要的业务事实，不包含响应或搜索引擎字段。
type MentionCandidate struct {
	ID            uint
	IsFriend      bool
	FollowerCount int64
}

type MentionOrder struct{}

func (MentionOrder) Less(a, b MentionCandidate) bool {
	if a.IsFriend != b.IsFriend {
		return a.IsFriend && !b.IsFriend
	}
	if a.FollowerCount != b.FollowerCount {
		return a.FollowerCount > b.FollowerCount
	}
	return a.ID < b.ID
}

type SuggestionCandidate struct{ Kind, Value string }
type SuggestionSelection struct{ Group, Index int }
type SuggestionPolicy struct{}

// Select 按来源顺序保留首次命中；主检索忽略大小写，数据库回退按大小写敏感方式去重。
func (SuggestionPolicy) Select(limit int, foldCase bool, groups ...[]SuggestionCandidate) []SuggestionSelection {
	selected := make([]SuggestionSelection, 0, limit)
	seen := make(map[string]struct{}, limit)
	for group, candidates := range groups {
		for index, candidate := range candidates {
			value := candidate.Value
			if foldCase {
				value = strings.ToLower(value)
			}
			key := candidate.Kind + "|" + value
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			selected = append(selected, SuggestionSelection{Group: group, Index: index})
			if len(selected) >= limit {
				return selected
			}
		}
	}
	return selected
}

const RelatedTagLimit = 10

type RelatedTagPolicy struct{}

// Select 保持命中顺序，去除空标签与重复标签，最多返回十个。
func (RelatedTagPolicy) Select(groups ...[]string) []string {
	selected := make([]string, 0)
	seen := make(map[string]struct{})
	for _, tags := range groups {
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, exists := seen[tag]; exists {
				continue
			}
			seen[tag] = struct{}{}
			selected = append(selected, tag)
			if len(selected) >= RelatedTagLimit {
				return selected
			}
		}
	}
	return selected
}
