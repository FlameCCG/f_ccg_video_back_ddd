package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

// 迁移自原 service/search_service/suggest_test.go 的应用层部分。
//
// 原 TestHighlightSuggestionMidStringAndChinese / TestEscapeESWildcard 随纯规则下移到
// internal/domain/search（见 keyword_test.go）；
// 原 TestIsPublishedVideoSuggestionRequiresPublishedStatus 随 `_source` 解码下移到
// internal/infrastructure/search（见 parse_test.go）；
// 原两个 sqlite 回退用例随 SQL 下移到 internal/infrastructure/persistence/search
// （见 suggest_fallback_test.go）。

func TestMergeSuggestionPriority(t *testing.T) {
	t.Parallel()

	got := mergeSuggestionPriority(3,
		[]SuggestionItem{{Value: "v1", DocType: DocTypeVideo}},
		[]SuggestionItem{{Value: "u1", DocType: DocTypeUser}, {Value: "v1", DocType: DocTypeVideo}},
		[]SuggestionItem{{Value: "t1", DocType: DocTypeTag}},
	)

	want := []string{"v1", "u1", "t1"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}

	for i := range want {
		if got[i].Value != want[i] {
			t.Fatalf("idx %d got %q want %q", i, got[i].Value, want[i])
		}
	}
}

// TestMergeSuggestionPriorityDedupIsCaseInsensitive 去重键是「类型 + 小写值」，
// 因此同类型下大小写不同的同一候选只保留第一个，不同类型的同名候选各保留一条。
func TestMergeSuggestionPriorityDedupIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	got := mergeSuggestionPriority(10,
		[]SuggestionItem{{Value: "Alice", DocType: DocTypeUser}},
		[]SuggestionItem{{Value: "alice", DocType: DocTypeUser}, {Value: "alice", DocType: DocTypeTag}},
	)

	if len(got) != 2 {
		t.Fatalf("items = %+v, want 2 entries", got)
	}

	if got[0].Value != "Alice" || got[1].DocType != DocTypeTag {
		t.Fatalf("items = %+v", got)
	}
}

// TestSuggestReturnsEmptyForBlankPrefix 空白前缀直接返回空切片，不查 ES、不写缓存。
func TestSuggestReturnsEmptyForBlankPrefix(t *testing.T) {
	t.Parallel()

	cache := newFakeSuggestionCache()
	svc := NewSuggestService(&fakeSuggestSearch{available: true}, &fakeSuggestFallback{available: true}, cache, zap.NewNop())

	items, err := svc.Suggest(context.Background(), "   ", 10)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if items == nil || len(items) != 0 {
		t.Fatalf("items = %v, want empty slice", items)
	}

	if cache.stores != 0 {
		t.Fatalf("blank prefix must not populate the cache (stores=%d)", cache.stores)
	}
}

// TestSuggestServesFromCacheWithoutQueryingES 命中缓存时不再查 ES。
func TestSuggestServesFromCacheWithoutQueryingES(t *testing.T) {
	t.Parallel()

	cache := newFakeSuggestionCache()
	cached := []SuggestionItem{{Value: "cached", DocType: DocTypeVideo}}
	cache.entries[suggestCacheKey("Go", 10)] = cached

	index := &fakeSuggestSearch{available: true, err: errors.New("must not be called")}
	svc := NewSuggestService(index, &fakeSuggestFallback{available: true}, cache, zap.NewNop())

	items, err := svc.Suggest(context.Background(), "Go", 10)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if !reflect.DeepEqual(items, cached) {
		t.Fatalf("items = %+v, want %+v", items, cached)
	}
}

// TestSuggestCacheKeyIgnoresCaseAndSurroundingSpace 缓存键按小写前缀 + size 归一。
func TestSuggestCacheKeyIgnoresCaseAndSurroundingSpace(t *testing.T) {
	t.Parallel()

	if got, want := suggestCacheKey("  GoLang ", 10), "golang|10"; got != want {
		t.Fatalf("suggestCacheKey = %q, want %q", got, want)
	}
}

// TestSuggestMergesThreeDimensionsByPriority ES 可用时按 title > username > tag 合并。
func TestSuggestMergesThreeDimensionsByPriority(t *testing.T) {
	t.Parallel()

	index := &fakeSuggestSearch{
		available: true,
		items: map[string][]SuggestionItem{
			DocTypeVideo: {{Value: "v", DocType: DocTypeVideo}},
			DocTypeUser:  {{Value: "u", DocType: DocTypeUser}},
			DocTypeTag:   {{Value: "t", DocType: DocTypeTag}},
		},
	}
	cache := newFakeSuggestionCache()
	svc := NewSuggestService(index, &fakeSuggestFallback{available: true}, cache, zap.NewNop())

	items, err := svc.Suggest(context.Background(), "x", 10)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if got := suggestValues(items); !reflect.DeepEqual(got, []string{"v", "u", "t"}) {
		t.Fatalf("values = %v, want [v u t]", got)
	}

	if cache.stores != 1 {
		t.Fatalf("stores = %d, want 1", cache.stores)
	}
}

// TestSuggestFallsBackToDatabaseWhenESFails ES 出错时回退 MySQL，并缓存回退结果。
func TestSuggestFallsBackToDatabaseWhenESFails(t *testing.T) {
	t.Parallel()

	index := &fakeSuggestSearch{available: true, err: errors.New("es down")}
	fallback := &fakeSuggestFallback{
		available: true,
		titles:    []string{"标题"},
		usernames: []string{"alice"},
		tags:      []string{"教程"},
	}
	cache := newFakeSuggestionCache()
	svc := NewSuggestService(index, fallback, cache, zap.NewNop())

	items, err := svc.Suggest(context.Background(), "x", 10)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if got := suggestValues(items); !reflect.DeepEqual(got, []string{"标题", "alice", "教程"}) {
		t.Fatalf("values = %v", got)
	}

	if cache.stores != 1 {
		t.Fatalf("stores = %d, want 1", cache.stores)
	}
}

// TestFallbackSuggestOnlyQueriesLaterDimensionsWhenShort 前一维度已填满 size 时，
// 后续维度不再查询（原实现按剩余额度递减 size）。
func TestFallbackSuggestOnlyQueriesLaterDimensionsWhenShort(t *testing.T) {
	t.Parallel()

	fallback := &fakeSuggestFallback{
		available: true,
		titles:    []string{"t1", "t2"},
		usernames: []string{"u1"},
		tags:      []string{"g1"},
	}
	svc := NewSuggestService(&fakeSuggestSearch{available: false}, fallback, newFakeSuggestionCache(), zap.NewNop())

	items, err := svc.Suggest(context.Background(), "x", 2)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if got := suggestValues(items); !reflect.DeepEqual(got, []string{"t1", "t2"}) {
		t.Fatalf("values = %v, want [t1 t2]", got)
	}

	if _, queried := fallback.sizes["usernames"]; queried {
		t.Fatal("usernames dimension must be skipped once size is satisfied")
	}
}

// TestFallbackSuggestPassesRemainingSizeToLaterDimensions 额度不足时把剩余额度传给下一维度。
func TestFallbackSuggestPassesRemainingSizeToLaterDimensions(t *testing.T) {
	t.Parallel()

	fallback := &fakeSuggestFallback{
		available: true,
		titles:    []string{"t1"},
		usernames: []string{"u1"},
		tags:      []string{"g1"},
	}
	svc := NewSuggestService(&fakeSuggestSearch{available: false}, fallback, newFakeSuggestionCache(), zap.NewNop())

	if _, err := svc.Suggest(context.Background(), "x", 3); err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if got := fallback.sizes["usernames"]; got != 2 {
		t.Fatalf("usernames size = %d, want 2", got)
	}

	if got := fallback.sizes["tags"]; got != 1 {
		t.Fatalf("tags size = %d, want 1", got)
	}
}

// TestSuggestClampsSize size 超上限截到 20，缺省补 10（缓存键也随之归一）。
func TestSuggestClampsSize(t *testing.T) {
	t.Parallel()

	fallback := &fakeSuggestFallback{available: true}
	cache := newFakeSuggestionCache()
	svc := NewSuggestService(&fakeSuggestSearch{available: false}, fallback, cache, zap.NewNop())

	if _, err := svc.Suggest(context.Background(), "x", 1_000); err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if got := fallback.sizes["titles"]; got != 20 {
		t.Fatalf("clamped size = %d, want 20", got)
	}

	if _, ok := cache.entries[suggestCacheKey("x", 20)]; !ok {
		t.Fatalf("cache keys = %v", cache.entries)
	}
}

// TestSuggestReturnsEmptyWhenBothESAndDBUnavailable 两侧都不可用时返回空切片而非 nil。
func TestSuggestReturnsEmptyWhenBothESAndDBUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewSuggestService(
		&fakeSuggestSearch{available: false},
		&fakeSuggestFallback{available: false},
		newFakeSuggestionCache(), zap.NewNop())

	items, err := svc.Suggest(context.Background(), "x", 10)
	if err != nil {
		t.Fatalf("Suggest: %v", err)
	}

	if items == nil || len(items) != 0 {
		t.Fatalf("items = %v, want empty slice", items)
	}
}

// TestSuggestPropagatesFallbackError 回退查询报错时向上返回错误、不写缓存。
func TestSuggestPropagatesFallbackError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db down")
	cache := newFakeSuggestionCache()
	svc := NewSuggestService(
		&fakeSuggestSearch{available: false},
		&fakeSuggestFallback{available: true, err: wantErr},
		cache, zap.NewNop())

	if _, err := svc.Suggest(context.Background(), "x", 10); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	if cache.stores != 0 {
		t.Fatalf("failed fallback must not populate the cache (stores=%d)", cache.stores)
	}
}

// TestBuildSuggestionItemsDropsBlankValuesAndHighlights 空值被丢弃，其余带上高亮。
func TestBuildSuggestionItemsDropsBlankValuesAndHighlights(t *testing.T) {
	t.Parallel()

	items := buildSuggestionItems([]string{"HelloWorld", ""}, "hell", DocTypeVideo)
	if len(items) != 1 {
		t.Fatalf("items = %+v, want 1 entry", items)
	}

	if items[0].Highlight != "<em>Hell</em>oWorld" || items[0].DocType != DocTypeVideo {
		t.Fatalf("item = %+v", items[0])
	}
}

func suggestValues(items []SuggestionItem) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.Value)
	}

	return values
}
