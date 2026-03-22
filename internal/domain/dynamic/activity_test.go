package dynamic_test

import (
	"testing"
	"time"

	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

func TestFollowActivityLatest(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	later := base.Add(time.Hour)

	cases := []struct {
		name     string
		activity dynamicdomain.FollowActivity
		wantAt   time.Time
		wantType enum.WorkType
		wantOK   bool
	}{
		{
			name:     "两者都无作品",
			activity: dynamicdomain.FollowActivity{},
			wantOK:   false,
		},
		{
			name:     "只有视频",
			activity: dynamicdomain.FollowActivity{LatestVideoAt: base},
			wantAt:   base,
			wantType: enum.WorkTypeVideo,
			wantOK:   true,
		},
		{
			name:     "只有动态",
			activity: dynamicdomain.FollowActivity{LatestDynamicAt: base},
			wantAt:   base,
			wantType: enum.WorkTypeDynamic,
			wantOK:   true,
		},
		{
			name:     "动态更新",
			activity: dynamicdomain.FollowActivity{LatestVideoAt: base, LatestDynamicAt: later},
			wantAt:   later,
			wantType: enum.WorkTypeDynamic,
			wantOK:   true,
		},
		{
			name:     "视频更新",
			activity: dynamicdomain.FollowActivity{LatestVideoAt: later, LatestDynamicAt: base},
			wantAt:   later,
			wantType: enum.WorkTypeVideo,
			wantOK:   true,
		},
		{
			name:     "时间相等时算视频",
			activity: dynamicdomain.FollowActivity{LatestVideoAt: base, LatestDynamicAt: base},
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
		activity dynamicdomain.FollowActivity
		want     bool
	}{
		{name: "无作品则不算未读", activity: dynamicdomain.FollowActivity{}, want: false},
		{name: "从未读过", activity: dynamicdomain.FollowActivity{LatestVideoAt: base}, want: true},
		{
			name:     "已读位点早于最新作品",
			activity: dynamicdomain.FollowActivity{LatestDynamicAt: base, LastReadAt: base.Add(-time.Second)},
			want:     true,
		},
		{
			name:     "已读位点等于最新作品",
			activity: dynamicdomain.FollowActivity{LatestDynamicAt: base, LastReadAt: base},
			want:     false,
		},
		{
			name:     "已读位点晚于最新作品",
			activity: dynamicdomain.FollowActivity{LatestVideoAt: base, LastReadAt: base.Add(time.Second)},
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
		video    dynamicdomain.WorkRef
		dynamic  dynamicdomain.WorkRef
		wantOK   bool
		wantType enum.WorkType
		wantID   uint
	}{
		{name: "都不存在", wantOK: false},
		{
			name:     "只有视频",
			video:    dynamicdomain.WorkRef{ID: 7, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeVideo,
			wantID:   7,
		},
		{
			name:     "只有动态",
			dynamic:  dynamicdomain.WorkRef{ID: 9, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeDynamic,
			wantID:   9,
		},
		{
			name:     "动态更新",
			video:    dynamicdomain.WorkRef{ID: 7, CreatedAt: base},
			dynamic:  dynamicdomain.WorkRef{ID: 9, CreatedAt: later},
			wantOK:   true,
			wantType: enum.WorkTypeDynamic,
			wantID:   9,
		},
		{
			name:     "时间相等时算视频",
			video:    dynamicdomain.WorkRef{ID: 7, CreatedAt: base},
			dynamic:  dynamicdomain.WorkRef{ID: 9, CreatedAt: base},
			wantOK:   true,
			wantType: enum.WorkTypeVideo,
			wantID:   7,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := (dynamicdomain.FeedPolicy{}).PickLatestWork(tc.video, tc.dynamic)
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

func TestLatestWorkReadEligibility(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	video := dynamicdomain.LatestWork{WorkType: enum.WorkTypeVideo, WorkID: 7, CreatedAt: base}
	if !video.CanMarkRead(1, 2) {
		t.Fatal("complete video should be markable")
	}

	if !video.IsCurrentVideo(7) || video.IsCurrentVideo(8) {
		t.Fatal("current video identity drifted")
	}

	if video.CanMarkRead(0, 2) || video.CanMarkRead(1, 0) {
		t.Fatal("missing user or author should not be marked read")
	}

	if (dynamicdomain.LatestWork{WorkType: enum.WorkTypeVideo, CreatedAt: base}).CanMarkRead(1, 2) {
		t.Fatal("zero work id should not be markable")
	}

	if (dynamicdomain.LatestWork{WorkType: enum.WorkTypeVideo, WorkID: 7}).CanMarkRead(1, 2) {
		t.Fatal("zero publish time should not be markable")
	}

	post := dynamicdomain.LatestWork{WorkType: enum.WorkTypeDynamic, WorkID: 7, CreatedAt: base}
	if post.IsCurrentVideo(7) || !post.CanMarkRead(1, 2) {
		t.Fatal("dynamic must stay markable without counting as the current video")
	}
}
