package platform

import (
	"testing"
	"time"
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

func TestPlanAutoUnban(t *testing.T) {
	endAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)

	plan := PlanAutoUnban([]ExpiredBan{
		{RecordID: 1, UserID: 0, EndAt: &endAt},
		{RecordID: 0, UserID: 8, EndAt: &endAt},
		{RecordID: 7, UserID: 9, EndAt: &endAt},
	})

	if plan.Empty() {
		t.Fatal("plan should not be empty")
	}

	if len(plan.UserIDs) != 1 || plan.UserIDs[0] != 9 {
		t.Fatalf("expected only user 9, got %v", plan.UserIDs)
	}

	if len(plan.RecordIDs) != 1 || plan.RecordIDs[0] != 7 {
		t.Fatalf("expected only record 7, got %v", plan.RecordIDs)
	}
}

func TestPlanAutoUnbanEmptyInputs(t *testing.T) {
	if !PlanAutoUnban(nil).Empty() {
		t.Fatal("nil input should produce an empty plan")
	}

	// 全部为非法用户或记录 ID 时同样视为无事可做。
	if !PlanAutoUnban([]ExpiredBan{{RecordID: 1, UserID: 0}}).Empty() {
		t.Fatal("all-invalid input should produce an empty plan")
	}
}

func TestFoldCommentLikeIntentsKeepsLastIntent(t *testing.T) {
	intents, ok := FoldCommentLikeIntents([]CommentLikeEvent{
		{CommentID: 1, UserID: 2, Action: LikeActionLike},
		{CommentID: 1, UserID: 2, Action: LikeActionUnlike},
		{CommentID: 1, UserID: 3, Action: LikeActionLike},
	})
	if !ok {
		t.Fatal("expected foldable batch")
	}

	if len(intents) != 2 {
		t.Fatalf("expected 2 intents, got %d", len(intents))
	}

	if got := intents[CommentLikeIntentKey{CommentID: 1, UserID: 2}]; got != LikeActionUnlike {
		t.Fatalf("expected last intent unlike, got %q", got)
	}

	if got := intents[CommentLikeIntentKey{CommentID: 1, UserID: 3}]; got != LikeActionLike {
		t.Fatalf("expected like, got %q", got)
	}
}

func TestFoldCommentLikeIntentsDropsInvalidEvents(t *testing.T) {
	intents, ok := FoldCommentLikeIntents([]CommentLikeEvent{
		{CommentID: 0, UserID: 2, Action: LikeActionLike},
		{CommentID: 1, UserID: 0, Action: LikeActionLike},
		{CommentID: 1, UserID: 2, Action: "unknown"},
	})

	if ok {
		t.Fatal("batch of invalid events should report no applicable intent")
	}

	if len(intents) != 0 {
		t.Fatalf("expected no intents, got %d", len(intents))
	}
}
