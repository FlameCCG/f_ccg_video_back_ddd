package video

import "testing"

func TestFavoritePolicyFirstAdditionAndAuthorReward(t *testing.T) {
	policy := FavoritePolicy{}
	for _, test := range []struct {
		created bool
		existed bool
		first   bool
	}{{false, false, false}, {false, true, false}, {true, true, false}, {true, false, true}} {
		if got := policy.FirstAddition(test.created, test.existed); got != test.first {
			t.Fatalf("created=%t existed=%t: first=%t, want %t", test.created, test.existed, got, test.first)
		}
	}
	for _, test := range []struct {
		authorID uint
		actorID  uint
		reward   float64
	}{{0, 2, 0}, {2, 2, 0}, {1, 2, 0.1}, {1, 0, 0.1}} {
		if got := policy.AuthorReward(test.authorID, test.actorID); got != test.reward {
			t.Fatalf("author=%d actor=%d: reward=%v, want %v", test.authorID, test.actorID, got, test.reward)
		}
	}
}
