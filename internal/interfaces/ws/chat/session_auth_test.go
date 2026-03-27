package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"f_ccg_video_back_ddd/pkg/authsession"
)

func TestClientActionsRejectFailedValidationWithAccurateFeedback(t *testing.T) {
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
		for _, action := range []string{"send", "read"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				t.Parallel()
				ctx := authsession.WithValidator(t.Context(), func(context.Context) error { return tc.err })
				// No business service is wired: rejected actions must return before using one.
				c := &client{ctx: ctx, userID: 7, send: make(chan []byte, 1)}
				if action == "send" {
					c.handleSend(clientEvent{To: 8, Text: "hello"})
				} else {
					c.handleRead(clientEvent{PeerID: 8})
				}
				select {
				case data := <-c.send:
					var event WSEvent
					if err := json.Unmarshal(data, &event); err != nil {
						t.Fatal(err)
					}
					if event.Type != serverEventSystem || event.System != tc.message || event.Message != nil {
						t.Fatalf("unexpected feedback: %s", data)
					}
				default:
					t.Fatal("missing validation feedback")
				}
			})
		}
	}
}
