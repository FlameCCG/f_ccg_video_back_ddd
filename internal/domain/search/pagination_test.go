package search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePaginationBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", page: 0, pageSize: 0, wantPage: 1, wantPageSize: defaultPageSize},
		{name: "negative page", page: -3, pageSize: 5, wantPage: 1, wantPageSize: 5},
		{name: "over max", page: 2, pageSize: maxPageSize + 50, wantPage: 2, wantPageSize: maxPageSize},
		{name: "kept", page: 3, pageSize: 30, wantPage: 3, wantPageSize: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizePagination(tt.page, tt.pageSize)
			require.Equal(t, tt.wantPage, got.Page, "NormalizePagination(%d,%d) = %+v", tt.page, tt.pageSize, got)
			require.Equal(t, tt.wantPageSize, got.PageSize, "NormalizePagination(%d,%d) = %+v", tt.page, tt.pageSize, got)
		})
	}
}

func TestPaginationOffsetNeverNegative(t *testing.T) {
	t.Parallel()

	{
		got := (Pagination{Page: 1, PageSize: 20}).Offset()
		require.Equal(t, 0, got, "offset = %d, want 0", got)
	}

	{
		got := (Pagination{Page: 3, PageSize: 20}).Offset()
		require.Equal(t, 40, got, "offset = %d, want 40", got)
	}

	{
		got := (Pagination{Page: 0, PageSize: 20}).Offset()
		require.Equal(t, 0, got, "offset = %d, want 0", got)
	}
}

func TestNormalizeMentionAndHotKeywordPagination(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeMentionPagination(0, 0)
		require.Equal(t, defaultMentionPageSize, got.PageSize, "mention default = %+v", got)
	}

	{
		got := NormalizeMentionPagination(1, 500)
		require.Equal(t, maxMentionPageSize, got.PageSize, "mention max = %+v", got)
	}

	{
		got := NormalizeHotKeywordPagination(0, 0)
		require.Equal(t, defaultHotKeywordPageSize, got.PageSize, "hot keyword default = %+v", got)
	}

	{
		got := NormalizeHotKeywordPagination(1, 100)
		require.Equal(t, maxHotKeywordPageSize, got.PageSize, "hot keyword max = %+v", got)
	}
}

func TestNormalizeSuggestSize(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeSuggestSize(0)
		require.Equal(t, defaultSuggestSize, got, "size = %d, want %d", got, defaultSuggestSize)
	}

	{
		got := NormalizeSuggestSize(100)
		require.Equal(t, maxSuggestSize, got, "size = %d, want %d", got, maxSuggestSize)
	}

	{
		got := NormalizeSuggestSize(7)
		require.Equal(t, 7, got, "size = %d, want 7", got)
	}
}

func TestNormalizeVideoSortFallsBackToViews(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeVideoSort(99, SortOrderAsc)
		require.Equal(t, VideoSortViews, got.Field, "video sort = %+v", got)
		require.True(t, got.Ascending(), "video sort = %+v", got)
	}

	{
		got := NormalizeVideoSort(VideoSortDanmu, 7)
		require.Equal(t, VideoSortDanmu, got.Field, "only explicit 1 means ascending, got %+v", got)
		require.False(t, got.Ascending(), "only explicit 1 means ascending, got %+v", got)
	}
}

func TestNormalizeUserSortFallsBackToFollowers(t *testing.T) {
	t.Parallel()

	{
		got := NormalizeUserSort(-1, SortOrderDesc)
		require.Equal(t, UserSortFollowers, got.Field, "user sort = %+v", got)
	}

	{
		got := NormalizeUserSort(UserSortLevel, SortOrderAsc)
		require.Equal(t, UserSortLevel, got.Field, "user sort = %+v", got)
		require.True(t, got.Ascending(), "user sort = %+v", got)
	}
}

func TestHotKeywordSampleSize(t *testing.T) {
	t.Parallel()

	{
		got := HotKeywordSampleSize(1, 10)
		require.Equal(t, fallbackHotKeywordSampleMinSize, got, "sample size = %d, want %d", got, fallbackHotKeywordSampleMinSize)
	}

	{
		got := HotKeywordSampleSize(4, 20)
		require.Equal(t, 4*20*fallbackHotKeywordSampleMultiple, got, "sample size = %d", got)
	}
}

func TestSliceWindow(t *testing.T) {
	t.Parallel()

	start, end := SliceWindow(10, Pagination{Page: 2, PageSize: 4})
	require.Equal(t, 4, start, "window = (%d,%d), want (4,8)", start, end)
	require.Equal(t, 8, end, "window = (%d,%d), want (4,8)", start, end)

	start, end = SliceWindow(3, Pagination{Page: 2, PageSize: 4})
	require.Equal(t, start, end, "out-of-range window should be empty, got (%d,%d)", start, end)

	start, end = SliceWindow(10, Pagination{Page: 2, PageSize: 8})
	require.Equal(t, 8, start, "clamped window = (%d,%d), want (8,10)", start, end)
	require.Equal(t, 10, end, "clamped window = (%d,%d), want (8,10)", start, end)
}
