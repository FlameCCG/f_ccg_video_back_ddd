package media

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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

// TestTranscodeProgressKey 保持已有 Redis 进度 key：分P 与视频级、分P 集合必须可区分。
func TestTranscodeProgressKey(t *testing.T) {
	t.Parallel()

	cache := &ProgressCache{}
	if cache.progressKey(3, 0) == cache.progressKey(3, 1) {
		t.Fatal("part key should differ")
	}

	if cache.progressPartsKey(3) == cache.progressKey(3, 0) {
		t.Fatal("parts hash key must differ from the video-level key")
	}

	if cache.partFieldKey(7) != "7" {
		t.Fatalf("partFieldKey(7) = %q, want 7", cache.partFieldKey(7))
	}

	if got, want := cache.progressKey(3, 0), constants.TranscodeProgressKeyPrefix+"3"; got != want {
		t.Fatalf("video key = %q, want %q", got, want)
	}

	if got, want := cache.progressKey(3, 1), constants.TranscodeProgressKeyPrefix+"3:1"; got != want {
		t.Fatalf("part key = %q, want %q", got, want)
	}

	if got, want := cache.progressPartsKey(3), constants.TranscodeProgressKeyPrefix+"3:parts"; got != want {
		t.Fatalf("parts key = %q, want %q", got, want)
	}
}

func TestTranscodeProgressPartsKey(t *testing.T) {
	t.Parallel()
	cache := &ProgressCache{}
	if cache.progressPartsKey(3) == cache.progressKey(3, 0) {
		t.Fatal("parts hash key must differ from the video-level key")
	}
	if cache.partFieldKey(7) != "7" {
		t.Fatalf("part field = %q, want 7", cache.partFieldKey(7))
	}
}
