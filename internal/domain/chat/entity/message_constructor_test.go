package entity_test

import (
	"errors"
	"testing"
	"time"

	chaterrors "f_ccg_video_back_ddd/internal/domain/chat/domainerr"
	"f_ccg_video_back_ddd/internal/domain/chat/entity"
)

func TestNewMessageValidatesBeforeExposingState(t *testing.T) {
	msg, err := entity.NewMessage(1, 1, entity.MessageTypeText, "", "", nil, time.Time{})
	if msg != nil || !errors.Is(err, chaterrors.ErrSelfMessage) {
		t.Fatalf("participant error must precede content error: msg=%+v err=%v", msg, err)
	}
	media := &entity.Media{URL: "image.png"}
	msg, err = entity.NewMessage(9, 3, entity.MessageTypeImage, "", "", media, time.Time{})
	if err != nil || msg.ConversationID != "3_9" || msg.Preview() != "[图片]" {
		t.Fatalf("message identity and preview: msg=%+v err=%v", msg, err)
	}
	media.URL = "changed.png"
	if msg.Media.URL != "image.png" {
		t.Fatal("changing constructor input must not mutate message media")
	}
}
