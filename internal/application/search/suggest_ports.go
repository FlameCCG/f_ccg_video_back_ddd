package search

import (
	"context"
)

// SuggestSearch 搜索联想端口。
//
// docType 取 DocTypeVideo / DocTypeUser / DocTypeTag，适配器据此选择索引、
// 字段集合与「仅已发布」过滤条件。
type SuggestSearch interface {
	// Available 报告 ES 客户端是否就绪。
	Available() bool
	// FieldSuggest 基于 text/keyword/pinyin 倒排索引取联想候选，不依赖 completion FST。
	FieldSuggest(ctx context.Context, docType, prefix string, size int) ([]SuggestionItem, error)
}

// SuggestFallbackQuery ES 不可用时的联想回退查询端口.
type SuggestFallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// VideoTitles 取已发布视频标题（前缀命中优先，按最新去重标题排序）。
	VideoTitles(ctx context.Context, prefix string, size int) ([]string, error)
	// Usernames 取用户名（前缀命中优先）。
	Usernames(ctx context.Context, prefix string, size int) ([]string, error)
	// TagNames 取挂在已发布视频上的标签名（前缀命中优先）。
	TagNames(ctx context.Context, prefix string, size int) ([]string, error)
}

// SuggestionCache 联想结果的进程内短 TTL 缓存端口.
type SuggestionCache interface {
	// Load 读取缓存；未命中或已过期返回 false。
	Load(key string) ([]SuggestionItem, bool)
	// Store 写入缓存并刷新过期时间。
	Store(key string, items []SuggestionItem)
}
