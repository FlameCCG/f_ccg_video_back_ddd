package chat

import (
	"context"
	"testing"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 对应原 service/chat_service/ws_hub.go 中直接使用 global.RedisClient 的部分。
// Redis 频道格式 chat:user:{userId} 属于外部契约（多实例互通依赖它）。

// newTestRedis 启动 miniredis 并返回连接其上的客户端（离线测试，不依赖真实 Redis）。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	return server, client
}

func TestUserChannelFormat(t *testing.T) {
	t.Parallel()

	if got, want := userChannel(42), constants.ChatUserChannelPrefix+"42"; got != want {
		t.Fatalf("userChannel(42) = %q, want %q", got, want)
	}
}

// TestEventBusPublishAndSubscribeRoundTrip 发布的负载能被同频道订阅者原样收到。
func TestEventBusPublishAndSubscribeRoundTrip(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)
	bus := NewEventBus(client, zap.NewNop())

	if !bus.Available() {
		t.Fatal("bus with a client must be available")
	}

	received := make(chan string, 1)

	stop := bus.Subscribe(7, func(payload []byte) {
		select {
		case received <- string(payload):
		default:
		}
	})
	if stop == nil {
		t.Fatal("Subscribe must return a stop func when Redis is available")
	}

	defer stop()

	// 等订阅真正建立后再发布，避免消息落在订阅之前。
	waitForSubscribers(t, client, userChannel(7))

	bus.Publish(context.Background(), 7, []byte(`{"serverId":"a"}`))

	select {
	case got := <-received:
		if got != `{"serverId":"a"}` {
			t.Fatalf("payload = %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the published event")
	}
}

// TestEventBusSubscribeStopIsIdempotent 停止订阅可重复调用（房间销毁与 hub.Stop 都会调）。
func TestEventBusSubscribeStopIsIdempotent(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)
	bus := NewEventBus(client, zap.NewNop())

	stop := bus.Subscribe(7, func([]byte) {})
	if stop == nil {
		t.Fatal("expected a stop func")
	}

	for range 3 {
		stop()
	}
}

// TestEventBusDegradesWithoutRedis client 为 nil 时退化为单实例模式：
// Publish 静默跳过，Subscribe 返回 nil（调用方据此不做跨实例派发）。
func TestEventBusDegradesWithoutRedis(t *testing.T) {
	t.Parallel()

	bus := NewEventBus(nil, zap.NewNop())
	if bus.Available() {
		t.Fatal("nil client must report unavailable")
	}

	bus.Publish(context.Background(), 7, []byte("x"))

	if stop := bus.Subscribe(7, func([]byte) {}); stop != nil {
		t.Fatal("Subscribe must return nil when Redis is unavailable")
	}
}

// TestEventBusSkipsAnonymousAndNilHandler userID 为 0 不发布；handler 为 nil 不订阅。
func TestEventBusSkipsAnonymousAndNilHandler(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	bus := NewEventBus(client, zap.NewNop())

	bus.Publish(context.Background(), 0, []byte("x"))

	if got := server.PubSubChannels(constants.ChatUserChannelPrefix + "*"); len(got) != 0 {
		t.Fatalf("anonymous publish must not create channels: %v", got)
	}

	if stop := bus.Subscribe(7, nil); stop != nil {
		t.Fatal("nil handler must not subscribe")
	}
}

// TestEventBusPublishToleratesNilContext ctx 为 nil 时回落 context.TODO（与原实现一致）。
func TestEventBusPublishToleratesNilContext(t *testing.T) {
	t.Parallel()

	_, client := newTestRedis(t)

	//nolint:staticcheck // 显式覆盖 nil context 的兜底分支
	NewEventBus(client, zap.NewNop()).Publish(nil, 7, []byte("x"))
}

// TestEventBusPublishSwallowsRedisErrors 发布是最佳努力：Redis 挂掉只告警、不 panic。
func TestEventBusPublishSwallowsRedisErrors(t *testing.T) {
	t.Parallel()

	server, client := newTestRedis(t)
	bus := NewEventBus(client, zap.NewNop())

	server.Close()

	bus.Publish(context.Background(), 7, []byte("x"))
}

// waitForSubscribers 轮询直到目标频道出现订阅者。
func waitForSubscribers(t *testing.T, client *redis.Client, channel string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		counts, err := client.PubSubNumSub(context.Background(), channel).Result()
		if err == nil && counts[channel] > 0 {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("no subscriber appeared on %q", channel)
}
