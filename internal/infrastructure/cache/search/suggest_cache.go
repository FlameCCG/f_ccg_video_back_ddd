// Package search 是搜索限界上下文的缓存适配器：联想结果进程内缓存与 Redis 热搜词榜。
package search

import (
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
)

// suggestCacheTTL 联想结果的进程内缓存有效期.
const suggestCacheTTL = 30 * time.Second

type suggestCacheEntry struct {
	items     []app.SuggestionItem
	expiresAt time.Time
}

// SuggestionCache 联想结果的进程内短 TTL 缓存。
//
// 原实现是包级变量 + init() 初始化 map；此处改为可注入类型，
// 由组合根创建唯一实例，TTL 与「读出/写入均做拷贝」的语义保持不变。
type SuggestionCache struct {
	mu    sync.RWMutex
	items map[string]suggestCacheEntry
}

var _ app.SuggestionCache = (*SuggestionCache)(nil)

// NewSuggestionCache 构造联想结果缓存。
func NewSuggestionCache() *SuggestionCache {
	return &SuggestionCache{items: make(map[string]suggestCacheEntry)}
}

// Load 读取缓存；未命中或已过期返回 false，并顺手清掉过期条目。
func (c *SuggestionCache) Load(key string) ([]app.SuggestionItem, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}

		return nil, false
	}

	return cloneSuggestionItems(entry.items), true
}

// Store 写入缓存并刷新过期时间。
func (c *SuggestionCache) Store(key string, items []app.SuggestionItem) {
	c.mu.Lock()
	c.items[key] = suggestCacheEntry{
		items:     cloneSuggestionItems(items),
		expiresAt: time.Now().Add(suggestCacheTTL),
	}
	c.mu.Unlock()
}

// cloneSuggestionItems 拷贝候选列表，避免缓存内容被调用方就地修改。
func cloneSuggestionItems(items []app.SuggestionItem) []app.SuggestionItem {
	if len(items) == 0 {
		return []app.SuggestionItem{}
	}

	out := make([]app.SuggestionItem, len(items))
	copy(out, items)

	return out
}
