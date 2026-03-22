package video

import "testing"

func TestRankWindowPreservesTop100AndPartialPages(t *testing.T) {
	for _, tc := range []struct {
		total      int64
		page, size int
		want       RankWindow
	}{
		{200, 1, 30, RankWindow{100, 0, 30}},
		{200, 4, 30, RankWindow{100, 90, 10}},
		{200, 5, 30, RankWindow{Total: 100}},
		{12, 1, 50, RankWindow{12, 0, 12}},
		{0, 1, 20, RankWindow{}},
		{100, int(^uint(0) >> 1), 50, RankWindow{Total: 100}},
	} {
		if got := (ListingPolicy{}).RankWindow(tc.total, tc.page, tc.size); got != tc.want {
			t.Errorf("%+v: got=%+v", tc, got)
		}
	}
}

func TestLikeNotificationRecipientEligibility(t *testing.T) {
	for _, tc := range []struct {
		actor, author uint
		allowed       bool
	}{{1, 1, false}, {1, 0, false}, {1, 2, true}} {
		if got := (NoticePolicy{}).LikeRecipient(tc.actor, tc.author); got != tc.allowed {
			t.Errorf("recipient=%+v allowed=%v", tc, got)
		}
	}
}
