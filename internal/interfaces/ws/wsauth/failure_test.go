package wsauth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"f_ccg_video_back_ddd/pkg/authsession"

	"github.com/gorilla/websocket"
)

func TestValidationFailureFeedback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		err     error
		code    int
		message string
	}{
		{name: "deadline", err: fmt.Errorf("private Redis address: %w", context.DeadlineExceeded), code: websocket.CloseTryAgainLater, message: "session validation timed out, please retry"},
		{name: "transport timeout", err: &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}, code: websocket.CloseTryAgainLater, message: "session validation timed out, please retry"},
		{name: "invalid session", err: fmt.Errorf("credentials: %w", authsession.ErrInvalidSession), code: websocket.ClosePolicyViolation, message: "session expired, please sign in again"},
		{name: "storage failure", err: errors.New("private database credentials"), code: websocket.CloseTryAgainLater, message: "session validation unavailable, please retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FailureMessage(tc.err); got != tc.message {
				t.Fatalf("message=%q want=%q", got, tc.message)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Errorf("upgrade: %v", err)
					return
				}
				Close(conn, tc.err, time.Second)
			}))
			defer server.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, _, err = conn.ReadMessage()
			var closeErr *websocket.CloseError
			if !errors.As(err, &closeErr) || closeErr.Code != tc.code || closeErr.Text != tc.message {
				t.Fatalf("close=%v want code=%d message=%q", err, tc.code, tc.message)
			}
		})
	}
}
