package search

import (
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
)

// 联想结果缓存通过构造函数创建，每个实例独立持有缓存状态。

func TestSuggestionCacheStoreAndLoad(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()

	if _, ok := cache.Load("missing", 1); ok {
		t.Fatal("empty cache must miss")
	}

	items := []app.SuggestionItem{{Value: "教程", Highlight: "<em>教</em>程", DocType: app.DocTypeVideo}}
	cache.Store("k", 1, items)

	got, ok := cache.Load("k", 1)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if len(got) != 1 || got[0] != items[0] {
		t.Fatalf("cached items = %+v", got)
	}
}

// TestSuggestionCacheCopiesOnStoreAndLoad 读写都做拷贝，缓存内容不会被调用方就地改写。
func TestSuggestionCacheCopiesOnStoreAndLoad(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	items := []app.SuggestionItem{{Value: "原值", DocType: app.DocTypeVideo}}
	cache.Store("k", 1, items)

	// 改写调用方持有的切片，不应影响缓存。
	items[0].Value = "被改写"

	got, ok := cache.Load("k", 1)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if got[0].Value != "原值" {
		t.Fatalf("store must copy, got %q", got[0].Value)
	}

	// 改写取出的切片，也不应影响缓存。
	got[0].Value = "又被改写"

	again, _ := cache.Load("k", 1)
	if again[0].Value != "原值" {
		t.Fatalf("load must copy, got %q", again[0].Value)
	}
}

// TestSuggestionCacheEmptyListRoundTrips 空结果也会被缓存（避免反复打 ES），
// 且读出的是空切片而非 nil。
func TestSuggestionCacheEmptyListRoundTrips(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	cache.Store("k", 1, nil)

	got, ok := cache.Load("k", 1)
	if !ok {
		t.Fatal("empty result must still be cached")
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got = %v, want empty slice", got)
	}
}

// TestSuggestionCacheExpiresAndEvicts 过期条目视为未命中，并被顺手清除。
func TestSuggestionCacheExpiresAndEvicts(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	key := cache.cacheKey("k", 1)
	cache.items[key] = suggestCacheEntry{
		items:     []app.SuggestionItem{{Value: "stale"}},
		expiresAt: time.Now().Add(-time.Second),
	}

	if _, ok := cache.Load("k", 1); ok {
		t.Fatal("expired entry must miss")
	}

	if _, exists := cache.items[key]; exists {
		t.Fatal("expired entry must be evicted on load")
	}
}

// TestSuggestionCacheStoreRefreshesTTL 重复写入会刷新过期时间。
func TestSuggestionCacheStoreRefreshesTTL(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	cache.items[cache.cacheKey("k", 1)] = suggestCacheEntry{expiresAt: time.Now().Add(-time.Second)}

	cache.Store("k", 1, []app.SuggestionItem{{Value: "fresh"}})

	got, ok := cache.Load("k", 1)
	if !ok {
		t.Fatal("re-store must refresh the entry")
	}

	if got[0].Value != "fresh" {
		t.Fatalf("cached value = %q", got[0].Value)
	}
}

// TestSuggestionCacheConcurrentAccess 并发读写不触发 -race 告警。
func TestSuggestionCacheConcurrentAccess(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	done := make(chan struct{})

	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()

			for i := range 200 {
				cache.Store("k", 1, []app.SuggestionItem{{Value: "v", DocType: app.DocTypeVideo}})
				_, _ = cache.Load("k", 1)

				if i%50 == 0 {
					_, _ = cache.Load("other", 1)
				}
			}
		}()
	}

	for range 4 {
		<-done
	}
}

// TestSuggestCacheTTLMatchesBaseline 验证缓存 TTL 为 30s。
func TestSuggestCacheTTLMatchesBaseline(t *testing.T) {
	t.Parallel()

	if suggestCacheTTL != 30*time.Second {
		t.Fatalf("suggestCacheTTL = %v, want 30s", suggestCacheTTL)
	}
}

func TestSuggestCacheKeyIgnoresCaseAndSurroundingSpace(t *testing.T) {
	t.Parallel()

	if got, want := NewSuggestionCache().cacheKey("  GoLang ", 10), "golang|10"; got != want {
		t.Fatalf("cacheKey = %q, want %q", got, want)
	}
}
