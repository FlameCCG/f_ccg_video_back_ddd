package search

import "testing"

func TestNormalizePaginationBounds(t *testing.T) {
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
			got := NormalizePagination(tt.page, tt.pageSize)
			if got.Page != tt.wantPage || got.PageSize != tt.wantPageSize {
				t.Fatalf("NormalizePagination(%d,%d) = %+v", tt.page, tt.pageSize, got)
			}
		})
	}
}

func TestPaginationOffsetNeverNegative(t *testing.T) {
	if got := (Pagination{Page: 1, PageSize: 20}).Offset(); got != 0 {
		t.Fatalf("offset = %d, want 0", got)
	}

	if got := (Pagination{Page: 3, PageSize: 20}).Offset(); got != 40 {
		t.Fatalf("offset = %d, want 40", got)
	}

	if got := (Pagination{Page: 0, PageSize: 20}).Offset(); got != 0 {
		t.Fatalf("offset = %d, want 0", got)
	}
}

func TestNormalizeMentionAndHotKeywordPagination(t *testing.T) {
	if got := NormalizeMentionPagination(0, 0); got.PageSize != defaultMentionPageSize {
		t.Fatalf("mention default = %+v", got)
	}

	if got := NormalizeMentionPagination(1, 500); got.PageSize != maxMentionPageSize {
		t.Fatalf("mention max = %+v", got)
	}

	if got := NormalizeHotKeywordPagination(0, 0); got.PageSize != defaultHotKeywordPageSize {
		t.Fatalf("hot keyword default = %+v", got)
	}

	if got := NormalizeHotKeywordPagination(1, 100); got.PageSize != maxHotKeywordPageSize {
		t.Fatalf("hot keyword max = %+v", got)
	}
}

func TestNormalizeSuggestSize(t *testing.T) {
	if got := NormalizeSuggestSize(0); got != defaultSuggestSize {
		t.Fatalf("size = %d, want %d", got, defaultSuggestSize)
	}

	if got := NormalizeSuggestSize(100); got != maxSuggestSize {
		t.Fatalf("size = %d, want %d", got, maxSuggestSize)
	}

	if got := NormalizeSuggestSize(7); got != 7 {
		t.Fatalf("size = %d, want 7", got)
	}
}

func TestNormalizeVideoSortFallsBackToViews(t *testing.T) {
	if got := NormalizeVideoSort(99, SortOrderAsc); got.Field != VideoSortViews || !got.Ascending() {
		t.Fatalf("video sort = %+v", got)
	}

	if got := NormalizeVideoSort(VideoSortDanmu, 7); got.Field != VideoSortDanmu || got.Ascending() {
		t.Fatalf("only explicit 1 means ascending, got %+v", got)
	}
}

func TestNormalizeUserSortFallsBackToFollowers(t *testing.T) {
	if got := NormalizeUserSort(-1, SortOrderDesc); got.Field != UserSortFollowers {
		t.Fatalf("user sort = %+v", got)
	}

	if got := NormalizeUserSort(UserSortLevel, SortOrderAsc); got.Field != UserSortLevel || !got.Ascending() {
		t.Fatalf("user sort = %+v", got)
	}
}

func TestHotKeywordSampleSize(t *testing.T) {
	if got := HotKeywordSampleSize(1, 10); got != fallbackHotKeywordSampleMinSize {
		t.Fatalf("sample size = %d, want %d", got, fallbackHotKeywordSampleMinSize)
	}

	if got := HotKeywordSampleSize(4, 20); got != 4*20*fallbackHotKeywordSampleMultiple {
		t.Fatalf("sample size = %d", got)
	}
}

func TestSliceWindow(t *testing.T) {
	start, end := SliceWindow(10, Pagination{Page: 2, PageSize: 4})
	if start != 4 || end != 8 {
		t.Fatalf("window = (%d,%d), want (4,8)", start, end)
	}

	start, end = SliceWindow(3, Pagination{Page: 2, PageSize: 4})
	if start != end {
		t.Fatalf("out-of-range window should be empty, got (%d,%d)", start, end)
	}

	start, end = SliceWindow(10, Pagination{Page: 2, PageSize: 8})
	if start != 8 || end != 10 {
		t.Fatalf("clamped window = (%d,%d), want (8,10)", start, end)
	}
}
