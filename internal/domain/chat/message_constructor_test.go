package chat_test

import (
	"errors"
	"testing"
	"time"

	chatdomain "f_ccg_video_back_ddd/internal/domain/chat"
)

func TestNewMessageValidatesBeforeExposingState(t *testing.T) {
	msg, err := chatdomain.NewMessage(1, 1, chatdomain.MessageTypeText, "", "", nil, time.Time{})
	if msg != nil || !errors.Is(err, chatdomain.ErrSelfMessage) {
		t.Fatalf("participant error must precede content error: msg=%+v err=%v", msg, err)
	}
	media := &chatdomain.Media{URL: "image.png"}
	msg, err = chatdomain.NewMessage(9, 3, chatdomain.MessageTypeImage, "", "", media, time.Time{})
	if err != nil || msg.ConversationID() != "3_9" || msg.Preview() != "[图片]" {
		t.Fatalf("message identity and preview: msg=%+v err=%v", msg.Snapshot(), err)
	}
	media.URL = "changed.png"
	if msg.Media().URL != "image.png" {
		t.Fatal("changing constructor input must not mutate message media")
	}
	msg.Media().URL = "mutated.png"
	if msg.Media().URL != "image.png" {
		t.Fatal("Media() must return a copy")
	}
}
