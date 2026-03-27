// Package wsauth maps long-lived session validation failures to safe client feedback.
package wsauth

import (
	"errors"
	"time"

	"f_ccg_video_back_ddd/pkg/authsession"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/gorilla/websocket"
)

// FailureMessage only requests sign-in when credentials were actually rejected.
func FailureMessage(err error) string {
	if ctxutil.IsTimeout(err) {
		return "session validation timed out, please retry"
	}
	if errors.Is(err, authsession.ErrInvalidSession) {
		return "session expired, please sign in again"
	}
	return "session validation unavailable, please retry"
}

// Close rejects further traffic while distinguishing retryable verification failures
// from invalid credentials. Control frames are safe alongside the connection writer.
func Close(conn *websocket.Conn, err error, writeWait time.Duration) {
	code := websocket.CloseTryAgainLater
	if !ctxutil.IsTimeout(err) && errors.Is(err, authsession.ErrInvalidSession) {
		code = websocket.ClosePolicyViolation
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, FailureMessage(err)), time.Now().Add(writeWait))
	_ = conn.Close()
}
