package entity_test

import (
	"errors"
	"testing"
	"time"

	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	"f_ccg_video_back_ddd/internal/domain/comment/entity"
)

func TestCommentConstructionCannotSkipDraftValidation(t *testing.T) {
	comment, err := entity.NewComment(entity.Draft{UserID: 1, VideoID: 2, Content: "  "}, nil)
	if comment != nil || !errors.Is(err, commenterrors.ErrContentEmpty) {
		t.Fatalf("invalid draft produced a comment: comment=%+v err=%v", comment, err)
	}
	comment, err = entity.NewComment(entity.Draft{UserID: 1, VideoID: 2, Content: "  内容  "}, nil)
	if err != nil || comment.Content != "内容" {
		t.Fatalf("constructor must normalize content: comment=%+v err=%v", comment, err)
	}
}

func TestCommentOwnsMutableConstructorInputs(t *testing.T) {
	parentID, rootID := uint(3), uint(2)
	draft := entity.Draft{UserID: 1, VideoID: 2, Content: "内容", ParentID: &parentID, AtUserIDs: []uint{4}, Pictures: []string{"image.png"}}
	comment, err := entity.NewComment(draft, &rootID)
	if err != nil {
		t.Fatal(err)
	}
	parentID, rootID = 9, 9
	draft.AtUserIDs[0], draft.Pictures[0] = 9, "changed.png"
	if *comment.ParentID != 3 || *comment.RootID != 2 || comment.AtUserIDs[0] != 4 || comment.Pictures[0] != "image.png" {
		t.Fatalf("external input mutation changed comment: %+v", comment)
	}
}

func TestCommentPinChangesKeepFlagAndTimestampTogether(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	comment := &entity.Comment{ID: 1, VideoID: 2}
	if err := comment.SetPinned(true, now); err != nil || !comment.IsPinned || comment.PinnedAt == nil || !comment.PinnedAt.Equal(now) {
		t.Fatalf("pin: comment=%+v err=%v", comment, err)
	}
	if err := comment.SetPinned(false, now.Add(time.Hour)); err != nil || comment.IsPinned || comment.PinnedAt != nil {
		t.Fatalf("unpin must clear timestamp: comment=%+v err=%v", comment, err)
	}
	rootID := uint(1)
	reply := &entity.Comment{VideoID: 2, RootID: &rootID}
	if err := reply.SetPinned(true, now); !errors.Is(err, commenterrors.ErrPinRootOnly) || reply.IsPinned || reply.PinnedAt != nil {
		t.Fatalf("rejected pin must leave state unchanged: comment=%+v err=%v", reply, err)
	}
}
