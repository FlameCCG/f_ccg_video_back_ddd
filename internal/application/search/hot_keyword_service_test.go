package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testDisplayAlice = "Alice"
	testAlice        = "alice"
	testVlog         = "vlog"
)

// 对应原 service/search_service/hot_keywords.go（原文件无单测，此处按分支补齐）。

// TestHotKeywordsReadsRedisZSetDescending Redis 榜单非空时按分数倒序取页，
// 分数四舍五入成整数，总量取 ZCARD。
func TestHotKeywordsReadsRedisZSetDescending(t *testing.T) {
	t.Parallel()

	store := &fakeHotKeywordStore{
		available: true,
		total:     42,
		entries: []HotKeywordEntry{
			{Member: "教程", Score: 9.6},
			{Member: testVlog, Score: 3.2},
		},
	}
	svc := NewHotKeywordService(store, &fakeHotKeywordFallback{available: true})

	items, total, err := svc.HotKeywords(context.Background(), 2, 10)
	require.NoError(t, err, "HotKeywords: %v", err)

	require.EqualValues(t, 42, total, "total = %d, want 42", total)

	want := []HotKeywordItem{{Keyword: "教程", Score: 10}, {Keyword: testVlog, Score: 3}}
	require.True(t, reflect.DeepEqual(items, want), "items = %+v, want %+v", items, want)

	// page=2、pageSize=10 → ZREVRANGE 的闭区间 [10, 19]。
	require.Equal(t, [2]int64{10, 19}, store.lastRange, "range = %v, want [10 19]", store.lastRange)
}

// TestHotKeywordsSkipsBlankMembers 榜单里的空白成员被丢弃（不入结果），总量仍取 ZCARD。
func TestHotKeywordsSkipsBlankMembers(t *testing.T) {
	t.Parallel()

	store := &fakeHotKeywordStore{
		available: true,
		total:     2,
		entries:   []HotKeywordEntry{{Member: "   ", Score: 5}, {Member: "教程", Score: 1}},
	}
	svc := NewHotKeywordService(store, &fakeHotKeywordFallback{available: true})

	items, total, err := svc.HotKeywords(context.Background(), 1, 10)
	require.NoError(t, err, "HotKeywords: %v", err)

	require.EqualValues(t, 2, total, "items = %+v, total = %d", items, total)
	require.Len(t, items, 1, "items = %+v, total = %d", items, total)
	require.Equal(t, "教程", items[0].Keyword, "items = %+v, total = %d", items, total)
}

// TestHotKeywordsFallsBackWhenRedisUnavailableOrEmpty Redis 不可用或榜单为空时，
// 回退到热门视频标题；总量为去重后的样本数。
func TestHotKeywordsFallsBackWhenRedisUnavailableOrEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		store *fakeHotKeywordStore
	}{
		{name: "redis unavailable", store: &fakeHotKeywordStore{available: false}},
		{name: "empty zset", store: &fakeHotKeywordStore{available: true, total: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fallback := &fakeHotKeywordFallback{
				available: true,
				rows: []HotVideoTitle{
					{Title: "  教程  ", Views: 100},
					{Title: "教程", Views: 50},  // 规范化后与上一条重复，应去重
					{Title: "   ", Views: 10}, // 空白标题丢弃
					{Title: testVlog, Views: 20},
				},
			}
			svc := NewHotKeywordService(tt.store, fallback)

			items, total, err := svc.HotKeywords(context.Background(), 1, 10)
			require.NoError(t, err, "HotKeywords: %v", err)

			want := []HotKeywordItem{{Keyword: "教程", Score: 100}, {Keyword: testVlog, Score: 20}}
			require.True(t, reflect.DeepEqual(items, want), "items = %+v, want %+v", items, want)

			require.EqualValues(t, 2, total, "total = %d, want 2", total)

			// 采样行数至少 50（page*pageSize*5 = 50）。
			require.Equal(t, 50, fallback.lastSample, "sample size = %d, want 50", fallback.lastSample)
		})
	}
}

// TestHotKeywordsFallbackPaginatesInMemory 回退路径在内存里分页；越界页返回空切片。
func TestHotKeywordsFallbackPaginatesInMemory(t *testing.T) {
	t.Parallel()

	rows := []HotVideoTitle{
		{Title: "a", Views: 5},
		{Title: "b", Views: 4},
		{Title: "c", Views: 3},
	}
	fallback := &fakeHotKeywordFallback{available: true, rows: rows}
	svc := NewHotKeywordService(&fakeHotKeywordStore{available: false}, fallback)

	items, total, err := svc.HotKeywords(context.Background(), 2, 2)
	require.NoError(t, err, "HotKeywords: %v", err)

	require.EqualValues(t, 3, total, "items = %+v, total = %d", items, total)
	require.Len(t, items, 1, "items = %+v, total = %d", items, total)
	require.Equal(t, "c", items[0].Keyword, "items = %+v, total = %d", items, total)

	items, total, err = svc.HotKeywords(context.Background(), 9, 2)
	require.NoError(t, err, "HotKeywords: %v", err)

	require.EqualValues(t, 3, total, "out-of-range page: items = %v, total = %d", items, total)
	require.NotNil(t, items, "out-of-range page: items = %v, total = %d", items, total)
	require.Empty(t, items, "out-of-range page: items = %v, total = %d", items, total)
}

// TestHotKeywordsReturnsEmptyWhenBothSourcesUnavailable Redis 与 DB 都不可用时
// 返回空切片与 0，不报错（与原实现一致）。
func TestHotKeywordsReturnsEmptyWhenBothSourcesUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewHotKeywordService(&fakeHotKeywordStore{available: false}, &fakeHotKeywordFallback{available: false})

	items, total, err := svc.HotKeywords(context.Background(), 1, 10)
	require.NoError(t, err, "HotKeywords: %v", err)

	require.EqualValues(t, 0, total, "items = %v, total = %d", items, total)
	require.NotNil(t, items, "items = %v, total = %d", items, total)
	require.Empty(t, items, "items = %v, total = %d", items, total)
}

func TestHotKeywordsPropagatesRedisErrors(t *testing.T) {
	t.Parallel()

	cardErr := errors.New("zcard failed")
	svc := NewHotKeywordService(
		&fakeHotKeywordStore{available: true, cardErr: cardErr},
		&fakeHotKeywordFallback{available: true},
	)

	{
		_, _, err := svc.HotKeywords(context.Background(), 1, 10)
		require.ErrorIs(t, err, cardErr, "err = %v, want %v", err, cardErr)
	}

	rangeErr := errors.New("zrevrange failed")
	svc = NewHotKeywordService(
		&fakeHotKeywordStore{available: true, total: 5, rangeErr: rangeErr},
		&fakeHotKeywordFallback{available: true},
	)

	{
		_, _, err := svc.HotKeywords(context.Background(), 1, 10)
		require.ErrorIs(t, err, rangeErr, "err = %v, want %v", err, rangeErr)
	}
}

// TestRecordHotKeywordNormalizesAndSkipsBlank 入榜前规范化关键词；空白词与 Redis
// 不可用时都不写入。
func TestRecordHotKeywordNormalizesAndSkipsBlank(t *testing.T) {
	t.Parallel()

	store := &fakeHotKeywordStore{available: true}
	svc := NewHotKeywordService(store, &fakeHotKeywordFallback{})

	svc.RecordHotKeyword(context.Background(), "  多  空格   词 ")
	svc.RecordHotKeyword(context.Background(), "   ")

	require.True(t, reflect.DeepEqual(store.incremented, []string{"多 空格 词"}), "incremented = %v", store.incremented)

	unavailable := &fakeHotKeywordStore{available: false}
	NewHotKeywordService(unavailable, &fakeHotKeywordFallback{}).
		RecordHotKeyword(context.Background(), "教程")

	require.Empty(t, unavailable.incremented, "unavailable store must not record: %v", unavailable.incremented)
}
