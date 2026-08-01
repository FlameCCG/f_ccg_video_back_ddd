package search

import (
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
)

// 对应原 service/search_service/cache.go（原为包级 map + init()，此处为可注入类型）。

func TestSuggestionCacheStoreAndLoad(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()

	if _, ok := cache.Load("missing"); ok {
		t.Fatal("empty cache must miss")
	}

	items := []app.SuggestionItem{{Value: "教程", Highlight: "<em>教</em>程", DocType: app.DocTypeVideo}}
	cache.Store("k", items)

	got, ok := cache.Load("k")
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
	cache.Store("k", items)

	// 改写调用方持有的切片，不应影响缓存。
	items[0].Value = "被改写"

	got, ok := cache.Load("k")
	if !ok {
		t.Fatal("expected cache hit")
	}

	if got[0].Value != "原值" {
		t.Fatalf("store must copy, got %q", got[0].Value)
	}

	// 改写取出的切片，也不应影响缓存。
	got[0].Value = "又被改写"

	again, _ := cache.Load("k")
	if again[0].Value != "原值" {
		t.Fatalf("load must copy, got %q", again[0].Value)
	}
}

// TestSuggestionCacheEmptyListRoundTrips 空结果也会被缓存（避免反复打 ES），
// 且读出的是空切片而非 nil。
func TestSuggestionCacheEmptyListRoundTrips(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	cache.Store("k", nil)

	got, ok := cache.Load("k")
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
	cache.items["k"] = suggestCacheEntry{
		items:     []app.SuggestionItem{{Value: "stale"}},
		expiresAt: time.Now().Add(-time.Second),
	}

	if _, ok := cache.Load("k"); ok {
		t.Fatal("expired entry must miss")
	}

	if _, exists := cache.items["k"]; exists {
		t.Fatal("expired entry must be evicted on load")
	}
}

// TestSuggestionCacheStoreRefreshesTTL 重复写入会刷新过期时间。
func TestSuggestionCacheStoreRefreshesTTL(t *testing.T) {
	t.Parallel()

	cache := NewSuggestionCache()
	cache.items["k"] = suggestCacheEntry{expiresAt: time.Now().Add(-time.Second)}

	cache.Store("k", []app.SuggestionItem{{Value: "fresh"}})

	got, ok := cache.Load("k")
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
				cache.Store("k", []app.SuggestionItem{{Value: "v", DocType: app.DocTypeVideo}})
				_, _ = cache.Load("k")

				if i%50 == 0 {
					_, _ = cache.Load("other")
				}
			}
		}()
	}

	for range 4 {
		<-done
	}
}

// TestSuggestCacheTTLMatchesBaseline TTL 属于行为契约：原实现为 30s。
func TestSuggestCacheTTLMatchesBaseline(t *testing.T) {
	t.Parallel()

	if suggestCacheTTL != 30*time.Second {
		t.Fatalf("suggestCacheTTL = %v, want 30s", suggestCacheTTL)
	}
}
