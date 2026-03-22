package video

import (
	"testing"
)

func TestHotRankDecayPolicyDecay(t *testing.T) {
	policy := NewHotRankDecayPolicy()

	score, remove := policy.Decay(100)
	if remove {
		t.Fatal("high score should stay on the rank")
	}

	if score != 100*HotScoreDecayFactor {
		t.Fatalf("expected %v, got %v", 100*HotScoreDecayFactor, score)
	}

	// 衰减到阈值及以下即移出榜单。
	if _, remove := policy.Decay(HotScoreClearThreshold); !remove {
		t.Fatal("score at clear threshold should be removed")
	}
}

func TestHotRankDecayPolicyDecayAllKeepsOrder(t *testing.T) {
	policy := NewHotRankDecayPolicy()

	updates := policy.DecayAll([]HotRankEntry{
		{Member: "1", Score: 0.005},
		{Member: "2", Score: 50},
	})

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	if updates[0].Member != "1" || !updates[0].Remove {
		t.Fatalf("expected member 1 removed, got %+v", updates[0])
	}

	if updates[1].Member != "2" || updates[1].Remove {
		t.Fatalf("expected member 2 kept, got %+v", updates[1])
	}

	if policy.DecayAll(nil) != nil {
		t.Fatal("empty input should produce no updates")
	}
}

func TestHotRankDecayPolicyOverflowCount(t *testing.T) {
	policy := NewHotRankDecayPolicy()

	if got := policy.OverflowCount(HotRankMaxSize); got != 0 {
		t.Fatalf("expected 0 overflow at capacity, got %d", got)
	}

	if got := policy.OverflowCount(HotRankMaxSize + 7); got != 7 {
		t.Fatalf("expected 7 overflow, got %d", got)
	}
}
