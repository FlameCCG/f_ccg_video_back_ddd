package search

// HotKeywordItem 首页热搜关键词项.
type HotKeywordItem struct {
	Keyword string `json:"keyword"`
	Score   int64  `json:"score"`
}
