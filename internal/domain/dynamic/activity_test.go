package dynamic_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

func TestFollowActivityLatest(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	later := base.Add(time.Hour)

	cases := []struct {
		name     string
		activity domain.FollowActivity
		wantAt   time.Time
		wantType enum.WorkType
		wantOK   bool
	}{
		{
			name:     "两者都无作品",
			activity: domain.FollowActivity{},
			wantOK:   false,
		},
		{
			name:     "只有视频",
			activity: domain.FollowActivity{LatestVideoAt: base},
			wantAt:   base,
			wantType: enum.WorkTypeVideo,
			wantOK:   true,
		},
		{
			name:     "只有动态",
			activity: domain.FollowActivity{LatestDynamicAt: base},
			wantAt:   base,
			wantType: enum.WorkTypeDynamic,
			wantOK:   true,
		},
		{
			name:     "动态更新",
			activity: domain.FollowActivity{LatestVideoAt: base, LatestDynamicAt: later},
			wantAt:   later,
			wantType: enum.WorkTypeDynamic,
			wantOK:   true,
		},
		{
			name:     "视频更新",
			activity: domain.FollowActivity{LatestVideoAt: later, LatestDynamicAt: base},
			wantAt:   later,
			wantType: enum.WorkTypeVideo,
			wantOK:   true,
		},
		{
			name:     "时间相等时算视频",
			activity: domain.FollowActivity{LatestVideoAt: base, LatestDynamicAt: base},
			wantAt:   base,
			wantType: enum.WorkTypeVideo,
			wantOK:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			at, workType, ok := tc.activity.Latest()
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if !ok {
				return
			}

			if !at.Equal(tc.wantAt) {
				t.Errorf("at = %v, want %v", at, tc.wantAt)
			}

			if workType != tc.wantType {
				t.Errorf("workType = %v, want %v", workType, tc.wantType)
			}
		})
	}
}

func TestFollowActivityIsUnread(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		activity domain.FollowActivity
		want     bool
	}{
		{name: "无作品则不算未读", activity: domain.FollowActivity{}, want: false},
		{name: "从未读过", activity: domain.FollowActivity{LatestVideoAt: base}, want: true},
		{
			name:     "已读位点早于最新作品",
			activity: domain.FollowActivity{LatestDynamicAt: base, LastReadAt: base.Add(-time.Second)},
			want:     true,
		},
		{
			name:     "已读位点等于最新作品",
			activity: domain.FollowActivity{LatestDynamicAt: base, LastReadAt: base},
			want:     false,
		},
		{
			name:     "已读位点晚于最新作品",
			activity: domain.FollowActivity{LatestVideoAt: base, LastReadAt: base.Add(time.Second)},
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.activity.IsUnread(); got != tc.want {
				t.Errorf("IsUnread() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPickLatestWork(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	later := base.Add(time.Hour)

	cases := []struct {
		name     string
		video    domain.WorkRef
		dynamic  domain.WorkRef
		wantOK   bool
		wantType enum.WorkType
		wantID   uint
	}{
		{name: "都不存在", wantOK: false},
		{
			name:     "只有视频",
			video:    domain.WorkRef{ID: 7, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeVideo,
			wantID:   7,
		},
		{
			name:     "只有动态",
			dynamic:  domain.WorkRef{ID: 9, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeDynamic,
			wantID:   9,
		},
		{
			name:     "动态更新",
			video:    domain.WorkRef{ID: 7, CreatedAt: base},
			dynamic:  domain.WorkRef{ID: 9, CreatedAt: later},
			wantOK:   true,
			wantType: enum.WorkTypeDynamic,
			wantID:   9,
		},
		{
			name:     "时间相等时算视频",
			video:    domain.WorkRef{ID: 7, CreatedAt: base},
			dynamic:  domain.WorkRef{ID: 9, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeVideo,
			wantID:   7,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := domain.PickLatestWork(tc.video, tc.dynamic)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}

			if !ok {
				return
			}

			if got.WorkType != tc.wantType || got.WorkID != tc.wantID {
				t.Errorf("got = %+v, want type %v id %d", got, tc.wantType, tc.wantID)
			}
		})
	}
}
