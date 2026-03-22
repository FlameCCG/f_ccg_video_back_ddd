package comment

import (
	"testing"
)

func TestFoldCommentLikeIntentsKeepsLastIntent(t *testing.T) {
	intents, ok := (LikeSyncPolicy{}).FoldCommentLikeIntents([]CommentLikeEvent{
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
	intents, ok := (LikeSyncPolicy{}).FoldCommentLikeIntents([]CommentLikeEvent{
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
