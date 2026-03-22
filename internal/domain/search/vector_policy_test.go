package search

import "testing"

func TestRecallLimitPreservesConfiguredCap(t *testing.T) {
	for _, tc := range []struct{ requested, configured, want int }{{0, 8, 8}, {-1, 8, 8}, {3, 8, 3}, {20, 8, 8}, {3, 0, 3}, {0, 0, 0}} {
		if got := (VectorPolicy{}).RecallLimit(tc.requested, tc.configured); got != tc.want {
			t.Errorf("%+v got=%d", tc, got)
		}
	}
}
