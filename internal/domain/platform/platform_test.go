package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHotRankDecayPolicyDecay(t *testing.T) {
	t.Parallel()

	policy := NewHotRankDecayPolicy()

	score, remove := policy.Decay(100)
	require.False(t, remove, "high score should stay on the rank")

	require.InDelta(t, 100*HotScoreDecayFactor, score, 1e-9, "expected %v, got %v", 100*HotScoreDecayFactor, score)

	// 衰减到阈值及以下即移出榜单。
	{
		_, remove := policy.Decay(HotScoreClearThreshold)
		require.True(t, remove, "score at clear threshold should be removed")
	}
}

func TestHotRankDecayPolicyDecayAllKeepsOrder(t *testing.T) {
	t.Parallel()

	policy := NewHotRankDecayPolicy()

	updates := policy.DecayAll([]HotRankEntry{
		{Member: "1", Score: 0.005},
		{Member: "2", Score: 50},
	})

	require.Len(t, updates, 2, "expected 2 updates, got %d", len(updates))

	require.Equal(t, "1", updates[0].Member, "expected member 1 removed, got %+v", updates[0])
	require.True(t, updates[0].Remove, "expected member 1 removed, got %+v", updates[0])

	require.Equal(t, "2", updates[1].Member, "expected member 2 kept, got %+v", updates[1])
	require.False(t, updates[1].Remove, "expected member 2 kept, got %+v", updates[1])

	require.Nil(t, policy.DecayAll(nil), "empty input should produce no updates")
}

func TestHotRankDecayPolicyOverflowCount(t *testing.T) {
	t.Parallel()

	policy := NewHotRankDecayPolicy()

	{
		got := policy.OverflowCount(HotRankMaxSize)
		require.EqualValues(t, 0, got, "expected 0 overflow at capacity, got %d", got)
	}

	{
		got := policy.OverflowCount(HotRankMaxSize + 7)
		require.EqualValues(t, 7, got, "expected 7 overflow, got %d", got)
	}
}

func TestPlanAutoUnban(t *testing.T) {
	t.Parallel()

	endAt := time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC)

	plan := PlanAutoUnban([]ExpiredBan{
		{RecordID: 1, UserID: 0, EndAt: &endAt},
		{RecordID: 0, UserID: 8, EndAt: &endAt},
		{RecordID: 7, UserID: 9, EndAt: &endAt},
	})

	require.False(t, plan.Empty(), "plan should not be empty")

	require.Len(t, plan.UserIDs, 1, "expected only user 9, got %v", plan.UserIDs)
	require.EqualValues(t, 9, plan.UserIDs[0], "expected only user 9, got %v", plan.UserIDs)

	require.Len(t, plan.RecordIDs, 1, "expected only record 7, got %v", plan.RecordIDs)
	require.EqualValues(t, 7, plan.RecordIDs[0], "expected only record 7, got %v", plan.RecordIDs)
}

func TestPlanAutoUnbanEmptyInputs(t *testing.T) {
	t.Parallel()

	require.True(t, PlanAutoUnban(nil).Empty(), "nil input should produce an empty plan")

	// 全部为非法用户或记录 ID 时同样视为无事可做。
	require.True(t, PlanAutoUnban([]ExpiredBan{{RecordID: 1, UserID: 0}}).Empty(), "all-invalid input should produce an empty plan")
}

func TestFoldCommentLikeIntentsKeepsLastIntent(t *testing.T) {
	t.Parallel()

	intents, ok := FoldCommentLikeIntents([]CommentLikeEvent{
		{CommentID: 1, UserID: 2, Action: LikeActionLike},
		{CommentID: 1, UserID: 2, Action: LikeActionUnlike},
		{CommentID: 1, UserID: 3, Action: LikeActionLike},
	})
	require.True(t, ok, "expected foldable batch")

	require.Len(t, intents, 2, "expected 2 intents, got %d", len(intents))

	{
		got := intents[CommentLikeIntentKey{CommentID: 1, UserID: 2}]
		require.Equal(t, LikeActionUnlike, got, "expected last intent unlike, got %q", got)
	}

	{
		got := intents[CommentLikeIntentKey{CommentID: 1, UserID: 3}]
		require.Equal(t, LikeActionLike, got, "expected like, got %q", got)
	}
}

func TestFoldCommentLikeIntentsDropsInvalidEvents(t *testing.T) {
	t.Parallel()

	intents, ok := FoldCommentLikeIntents([]CommentLikeEvent{
		{CommentID: 0, UserID: 2, Action: LikeActionLike},
		{CommentID: 1, UserID: 0, Action: LikeActionLike},
		{CommentID: 1, UserID: 2, Action: "unknown"},
	})

	require.False(t, ok, "batch of invalid events should report no applicable intent")

	require.Empty(t, intents, "expected no intents, got %d", len(intents))
}
