package media

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRedisProgressPumpCloseUnblocksFullOutput(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { require.NoError(t, client.Close()) }()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, byContext := range []bool{false, true} {
		t.Run(map[bool]string{false: "Close", true: "context"}[byContext], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			pubsub := client.Subscribe(ctx, constants.TranscodeProgressChannel)
			{
				_, err := pubsub.Receive(ctx)
				require.NoError(t, err)
			}

			sub := newProgressSubscription(ctx, pubsub, zap.NewNop())

			for range 100 {
				{
					err := client.Publish(ctx, constants.TranscodeProgressChannel, `{"videoId":1,"status":"running"}`).Err()
					require.NoError(t, err)
				}
			}

			deadline := time.Now().Add(time.Second)
			for len(sub.events) < cap(sub.events) && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}

			require.Equal(t, len(sub.events), cap(sub.events), "pump did not reach backpressure")

			if byContext {
				cancel()
			} else {
				{
					err := sub.Close()
					require.NoError(t, err)
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
