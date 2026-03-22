package identity

import "testing"

func TestCoinExperienceRewardEligibilityAndConversion(t *testing.T) {
	for _, test := range []struct {
		coins    int64
		delta    int64
		eligible bool
	}{{-1, 0, false}, {0, 0, false}, {1, 15, true}, {2, 30, true}, {3, 45, true}} {
		delta, eligible := (ExperiencePolicy{}).CoinReward(test.coins)
		if delta != test.delta || eligible != test.eligible {
			t.Fatalf("coins=%d: got (%d, %t), want (%d, %t)", test.coins, delta, eligible, test.delta, test.eligible)
		}
	}
}

func TestFavoriteRewardRevokeEligibility(t *testing.T) {
	for _, test := range []struct {
		authorID uint
		actorID  uint
		eligible bool
	}{{0, 2, false}, {2, 2, false}, {1, 2, true}, {1, 0, true}} {
		if got := (FavoriteRewardPolicy{}).CanRevoke(test.authorID, test.actorID); got != test.eligible {
			t.Fatalf("author=%d actor=%d: eligible=%t, want %t", test.authorID, test.actorID, got, test.eligible)
		}
	}
}
