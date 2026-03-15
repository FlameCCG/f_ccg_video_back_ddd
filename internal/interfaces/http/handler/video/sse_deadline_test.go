package video

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/video"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

type deadlineSubscription struct {
	app.TranscodeProgressSubscription
	events chan app.TranscodeProgressItem
}

func (s deadlineSubscription) Events() <-chan app.TranscodeProgressItem { return s.events }
func (deadlineSubscription) Close() error                               { return nil }

type deadlineProgress struct{ app.TranscodeProgressReader }

func (deadlineProgress) Subscribe(context.Context) (app.TranscodeProgressSubscription, error) {
	return deadlineSubscription{events: make(chan app.TranscodeProgressItem)}, nil
}
func (deadlineProgress) MGetByVideoIDs(context.Context, []uint) ([]app.TranscodeProgressItem, error) {
	return nil, nil
}

func TestSSEExtendsNativeWriteDeadlineThroughGinWrapper(t *testing.T) {
	writer := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(writer)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	c.Request = httptest.NewRequestWithContext(ctx, "GET", "/progress?videoIds=1", nil)
	c.Set("request", AdminTranscodeProgressReq{VideoIDs: "1"})
	h := &AdminHandler{progress: deadlineProgress{}}
	h.AdminTranscodeProgressStreamView(c)
	if time.Until(writer.deadline) < 59*time.Minute {
		t.Fatalf("deadline was not extended: %v", writer.deadline)
	}
}
