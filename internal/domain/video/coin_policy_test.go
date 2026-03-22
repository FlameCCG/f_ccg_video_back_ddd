package video_test

import (
	"testing"

	video "f_ccg_video_back_ddd/internal/domain/video"
)

func TestCoinAuthorRewardReturnsCanonicalLedgerPrecision(t *testing.T) {
	for _, tc := range []struct {
		coins int64
		want  float64
	}{{-1, 0}, {0, 0}, {1, .1}, {2, .2}, {3, .3}} {
		if got := (video.CoinPolicy{}).AuthorReward(tc.coins); got != tc.want {
			t.Fatalf("coins=%d reward=%v want=%v", tc.coins, got, tc.want)
		}
	}
}
