package media

import (
	"context"
	domain "f_ccg_video_back_ddd/internal/domain/media"
	"sync"
	"testing"
	"time"
)

type progressInnerStub struct {
	events chan domain.TranscodeProgress
	once   sync.Once
}

func (s *progressInnerStub) Events() <-chan domain.TranscodeProgress { return s.events }
func (s *progressInnerStub) Close() error                            { s.once.Do(func() { close(s.events) }); return nil }
func TestProgressSubscriptionCloseUnblocksFullOutput(t *testing.T) {
	for _, cancelContext := range []bool{false, true} {
		t.Run(map[bool]string{false: "Close", true: "context"}[cancelContext], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			inner := &progressInnerStub{events: make(chan domain.TranscodeProgress, 128)}
			for i := 0; i < 128; i++ {
				inner.events <- domain.TranscodeProgress{VideoID: 1}
			}
			sub := newProgressSubscription(ctx, inner)
			deadline := time.Now().Add(time.Second)
			for len(sub.events) < cap(sub.events) && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if len(sub.events) != cap(sub.events) {
				t.Fatal("pump did not reach backpressure")
			}
			if cancelContext {
				cancel()
			} else {
				_ = sub.Close()
			}
			done := make(chan struct{})
			go func() {
				for range sub.Events() {
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("pump leaked")
			}
		})
	}
}
