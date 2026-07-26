package dynamic_test

import (
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

func TestBuildFeedSources(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		workType    int
		wantVideo   bool
		wantDynamic bool
		wantAny     bool
	}{
		{name: "0 取全部", workType: 0, wantVideo: true, wantDynamic: true, wantAny: true},
		{name: "1 只取视频", workType: 1, wantVideo: true, wantAny: true},
		{name: "2 只取动态", workType: 2, wantDynamic: true, wantAny: true},
		{name: "未知类型无来源", workType: 9, wantAny: false},
		{name: "负数无来源", workType: -1, wantAny: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := domain.BuildFeedSources(tc.workType)
			if got.IncludeVideo != tc.wantVideo || got.IncludeDynamic != tc.wantDynamic {
				t.Errorf("sources = %+v, want video=%v dynamic=%v", got, tc.wantVideo, tc.wantDynamic)
			}

			if got.Any() != tc.wantAny {
				t.Errorf("Any() = %v, want %v", got.Any(), tc.wantAny)
			}
		})
	}
}

func TestNormalizeFeedPaging(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "零值回落默认", wantPage: 1, wantPageSize: domain.DefaultFeedPageSize},
		{name: "负页码回落 1", page: -3, pageSize: 10, wantPage: 1, wantPageSize: 10},
		{name: "超上限截断", page: 2, pageSize: 500, wantPage: 2, wantPageSize: domain.MaxFeedPageSize},
		{name: "正常值保留", page: 3, pageSize: 30, wantPage: 3, wantPageSize: 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			page, pageSize := domain.NormalizeFeedPaging(tc.page, tc.pageSize)
			if page != tc.wantPage || pageSize != tc.wantPageSize {
				t.Errorf("got (%d, %d), want (%d, %d)", page, pageSize, tc.wantPage, tc.wantPageSize)
			}
		})
	}
}

func TestNormalizeAdminPaging(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "零值回落默认", wantPage: 1, wantPageSize: domain.DefaultAdminPageSize},
		{name: "超上限回落默认", page: 2, pageSize: 101, wantPage: 2, wantPageSize: domain.DefaultAdminPageSize},
		{name: "上限值保留", page: 1, pageSize: domain.MaxAdminPageSize, wantPage: 1, wantPageSize: domain.MaxAdminPageSize},
		{name: "正常值保留", page: 4, pageSize: 50, wantPage: 4, wantPageSize: 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			page, pageSize := domain.NormalizeAdminPaging(tc.page, tc.pageSize)
			if page != tc.wantPage || pageSize != tc.wantPageSize {
				t.Errorf("got (%d, %d), want (%d, %d)", page, pageSize, tc.wantPage, tc.wantPageSize)
			}
		})
	}
}

func TestLikeNoticeContent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "空正文兜底", content: "   ", want: domain.LikeNoticeFallbackContent},
		{name: "短正文原样", content: " 好看 ", want: "好看"},
		{
			name:    "超长截断",
			content: strings.Repeat("字", domain.LikeNoticeMaxRunes+5),
			want:    strings.Repeat("字", domain.LikeNoticeMaxRunes) + "...",
		},
		{
			name:    "边界不截断",
			content: strings.Repeat("字", domain.LikeNoticeMaxRunes),
			want:    strings.Repeat("字", domain.LikeNoticeMaxRunes),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := domain.LikeNoticeContent(tc.content); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
