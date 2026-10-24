package media

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestRedisProgressPumpCloseUnblocksFullOutput(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	for _, byContext := range []bool{false, true} {
		t.Run(map[bool]string{false: "Close", true: "context"}[byContext], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			pubsub := client.Subscribe(ctx, constants.TranscodeProgressChannel)
			if _, err := pubsub.Receive(ctx); err != nil {
				t.Fatal(err)
			}
			sub := newProgressSubscription(ctx, pubsub, zap.NewNop())
			for i := 0; i < 100; i++ {
				if err := client.Publish(ctx, constants.TranscodeProgressChannel, `{"videoId":1,"status":"running"}`).Err(); err != nil {
					t.Fatal(err)
				}
			}
			deadline := time.Now().Add(time.Second)
			for len(sub.events) < cap(sub.events) && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if len(sub.events) != cap(sub.events) {
				t.Fatal("pump did not reach backpressure")
			}
			if byContext {
				cancel()
			} else {
				if err := sub.Close(); err != nil {
					t.Fatal(err)
				}
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
				t.Fatal("Redis progress pump leaked")
			}
		})
	}
}
