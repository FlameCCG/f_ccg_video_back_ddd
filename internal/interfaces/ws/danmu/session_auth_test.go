package danmu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/authsession"
)

func TestSendRejectsFailedValidationWithAccurateFeedback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		err     error
		message string
	}{
		{name: "timeout", err: fmt.Errorf("private Redis address: %w", context.DeadlineExceeded), message: "session validation timed out, please retry"},
		{name: "unavailable", err: errors.New("private database credentials"), message: "session validation unavailable, please retry"},
		{name: "revoked", err: authsession.ErrInvalidSession, message: "session expired, please sign in again"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := authsession.WithValidator(t.Context(), func(context.Context) error { return tc.err })
			// An unwired service would fail if a rejected action reached the business layer.
			c := &client{ctx: ctx, userID: 7, videoID: 11, partID: 12, send: make(chan []byte, 1), room: &room{hub: &Hub{service: &videoapp.DanmuService{}}}}
			c.handleSend(clientMessage{Content: "hello"})
			select {
			case data := <-c.send:
				var event Event
				if err := json.Unmarshal(data, &event); err != nil {
					t.Fatal(err)
				}
				if event.Type != serverEventSystem || event.System != tc.message || event.VideoID != 11 || event.VideoPartID != 12 || event.Danmu != nil {
					t.Fatalf("unexpected feedback: %s", data)
				}
			default:
				t.Fatal("missing validation feedback")
			}
		})
	}
}
