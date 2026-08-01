package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// cancelPublishTimeout 广播取消事件的最长等待。
const cancelPublishTimeout = 2 * time.Second

// cancelEvent 是转码取消广播的消息体（Redis Pub/Sub 外部契约）。
type cancelEvent struct {
	VideoIDs    []uint    `json:"videoIds"`
	RequestedAt time.Time `json:"requestedAt"`
}

// CancelBroadcaster 通过 Redis Pub/Sub 向其它实例广播「视频已被硬删除」。
type CancelBroadcaster struct {
	redis *redis.Client
}

var _ domain.CancelBroadcaster = (*CancelBroadcaster)(nil)

// NewCancelBroadcaster 构造取消事件广播器。
func NewCancelBroadcaster(redisClient *redis.Client) *CancelBroadcaster {
	return &CancelBroadcaster{redis: redisClient}
}

// Available 报告底层 Redis 是否可用；不可用时调用方只做本地取消。
func (b *CancelBroadcaster) Available() bool {
	return b != nil && b.redis != nil
}

// PublishVideoDeleted 广播取消事件。
func (b *CancelBroadcaster) PublishVideoDeleted(ctx context.Context, videoIDs []uint) error {
	if !b.Available() {
		return nil
	}

	payload, err := json.Marshal(cancelEvent{VideoIDs: videoIDs, RequestedAt: time.Now()})
	if err != nil {
		return fmt.Errorf("marshal transcode cancel event: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	pubCtx, cancel := context.WithTimeout(ctx, cancelPublishTimeout)
	defer cancel()

	if err := b.redis.Publish(pubCtx, constants.TranscodeCancelChannel, payload).Err(); err != nil {
		return fmt.Errorf("publish transcode cancel event: %w", err)
	}

	return nil
}

// CancelSubscriber 监听取消广播并作用到本地任务登记表。
//
// 原实现为 transcode_cancel_service.Subscriber（依赖 global.RedisClient 与包级 DefaultRegistry），
// 新架构由构造函数注入 Redis 客户端与登记表。
type CancelSubscriber struct {
	client   *redis.Client
	registry *domain.TaskRegistry
	logger   *zap.Logger

	mu      sync.Mutex
	cancel  context.CancelFunc
	pubsub  *redis.PubSub
	started bool
	wg      sync.WaitGroup
}

// NewCancelSubscriber 创建进程级取消事件订阅者。
func NewCancelSubscriber(
	client *redis.Client,
	registry *domain.TaskRegistry,
	logger *zap.Logger,
) *CancelSubscriber {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CancelSubscriber{client: client, registry: registry, logger: logger}
}

// Start 在返回前完成订阅，避免消费者启动与监听就绪之间丢事件。
func (s *CancelSubscriber) Start(parent context.Context) error {
	if s == nil || s.client == nil || s.registry == nil {
		return nil
	}

	if parent == nil {
		parent = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	ctx, cancel := context.WithCancel(parent)

	pubsub := s.client.Subscribe(ctx, constants.TranscodeCancelChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		cancel()
		_ = pubsub.Close()

		return fmt.Errorf("subscribe transcode cancel events: %w", err)
	}

	s.cancel = cancel
	s.pubsub = pubsub
	s.started = true

	s.wg.Add(1)

	go s.run(ctx, pubsub)

	return nil
}

func (s *CancelSubscriber) run(ctx context.Context, pubsub *redis.PubSub) {
	defer s.wg.Done()

	channel := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-channel:
			if !ok {
				return
			}

			var event cancelEvent
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				s.logger.Warn("decode transcode cancel event failed", zap.Error(err))

				continue
			}

			canceled := s.registry.CancelVideos(event.VideoIDs)
			if canceled > 0 {
				s.logger.Info("remote transcode tasks canceled",
					zap.Int("tasks", canceled),
					zap.Ints("videoIds", uintsToInts(event.VideoIDs)),
				)
			}
		}
	}
}

// Stop 结束订阅并等待其 goroutine 退出。
func (s *CancelSubscriber) Stop() error {
	if s == nil {
		return nil
	}

	s.mu.Lock()

	if !s.started {
		s.mu.Unlock()

		return nil
	}

	cancel := s.cancel
	pubsub := s.pubsub

	if cancel != nil {
		cancel()
	}

	var closeErr error
	if pubsub != nil {
		closeErr = pubsub.Close()
	}

	s.wg.Wait()
	s.cancel = nil
	s.pubsub = nil
	s.started = false
	s.mu.Unlock()

	if closeErr != nil && !errors.Is(closeErr, redis.ErrClosed) {
		return fmt.Errorf("close transcode cancel subscriber: %w", closeErr)
	}

	return nil
}

func uintsToInts(values []uint) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		out = append(out, int(value))
	}

	return out
}
