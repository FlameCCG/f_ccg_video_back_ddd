// Package search 适配搜索联想进程内缓存与 Redis 热词榜。
//
// suggest_cache.go 与 hot_keyword_store.go 使用不同存储和失效方式；进程内命中不代表多实例共享。
// 缓存格式、key 和 TTL 与调用方契约配合，搜索和回退策略仍属于 application/search。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package search

import (
	"strconv"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	searchdomain "f_ccg_video_back_ddd/internal/domain/search"
)

// suggestCacheTTL 联想结果的进程内缓存有效期.
const suggestCacheTTL = 30 * time.Second

type suggestCacheEntry struct {
	items     []app.SuggestionItem
	expiresAt time.Time
}

// SuggestionCache 联想结果的进程内短 TTL 缓存。
//
// 由组合根创建唯一实例；读出与写入均复制数据，避免调用方修改共享状态。
type SuggestionCache struct {
	mu    sync.RWMutex
	items map[string]suggestCacheEntry
}

var _ app.SuggestionCache = (*SuggestionCache)(nil)

// NewSuggestionCache 构造联想结果缓存。
func NewSuggestionCache() *SuggestionCache {
	return &SuggestionCache{items: make(map[string]suggestCacheEntry)}
}

// Load 按前缀和候选数读取缓存；未命中或已过期返回 false，并顺手清掉过期条目。
func (c *SuggestionCache) Load(prefix string, size int) ([]app.SuggestionItem, bool) {
	key := c.cacheKey(prefix, size)
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

	return c.cloneItems(entry.items), true
}

// Store 按前缀和候选数写入缓存并刷新过期时间。
func (c *SuggestionCache) Store(prefix string, size int, items []app.SuggestionItem) {
	c.mu.Lock()
	c.items[c.cacheKey(prefix, size)] = suggestCacheEntry{
		items:     c.cloneItems(items),
		expiresAt: time.Now().Add(suggestCacheTTL),
	}
	c.mu.Unlock()
}

// cacheKey 生成联想缓存键（小写前缀 + 候选数）。
func (c *SuggestionCache) cacheKey(prefix string, size int) string {
	return (searchdomain.KeywordPolicy{}).LowerKeyword(prefix) + "|" + strconv.Itoa(size)
}

// cloneItems 拷贝候选列表，避免缓存内容被调用方就地修改。
func (c *SuggestionCache) cloneItems(items []app.SuggestionItem) []app.SuggestionItem {
	if len(items) == 0 {
		return []app.SuggestionItem{}
	}

	out := make([]app.SuggestionItem, len(items))
	copy(out, items)

	return out
}
