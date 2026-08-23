package mq

import (
	"context"
	"strings"
	"sync"
	"time"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	"f_ccg_video_back_ddd/internal/config"
	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

const (
	// transcodeConsumerInitialBackoff 首次失败后的等待时间。
	transcodeConsumerInitialBackoff = 2 * time.Second
	// transcodeConsumerMaxBackoff 指数退避上限。
	transcodeConsumerMaxBackoff = 30 * time.Second
	// transcodeConsumerMaxWait 从首次尝试起最长等待窗口（覆盖 Kafka 脏重启恢复）。
	transcodeConsumerMaxWait = 15 * time.Minute
)

// TranscodeConsumerComponent 是转码消费者的可注入后台组件（原 core/init_transcode_consumer.go）。
//
// 原实现用包级 sync.Once + 三个包级变量维护单例；新架构改为结构体字段 + 构造函数注入，
// 由 bootstrap 创建唯一实例并纳入 App 生命周期。启动语义逐字保留：
//   - Kafka 未启用只记日志跳过；
//   - 连接类失败在后台指数退避重试（2s→4s→8s→16s→30s 封顶，总窗口 15 分钟）；
//   - 配置类错误视为永久失败，不重试；
//   - Stop 取消在途重试并优雅关闭已启动的消费者。
type TranscodeConsumerComponent struct {
	cfg        *config.Kafka
	service    *mediaapp.TranscodeService
	registry   *media.TaskRegistry
	subscriber CancelSubscriberPort
	producers  *ProducerPool
	logger     *zap.Logger

	mu          sync.Mutex
	started     bool
	consumer    *TranscodeConsumer
	retryCancel context.CancelFunc
}

// NewTranscodeConsumerComponent 构造转码消费者后台组件。
func NewTranscodeConsumerComponent(
	cfg *config.Kafka,
	service *mediaapp.TranscodeService,
	registry *media.TaskRegistry,
	subscriber CancelSubscriberPort,
	producers *ProducerPool,
	logger *zap.Logger,
) *TranscodeConsumerComponent {
	return &TranscodeConsumerComponent{
		cfg:        cfg,
		service:    service,
		registry:   registry,
		subscriber: subscriber,
		producers:  producers,
		logger:     logger,
	}
}

// Start 在应用启动时按需启动转码消费者。
//
// 连接类失败会在后台指数退避重试，不阻塞 HTTP 服务启动；因此本方法总是立即返回 nil
// （与原 InitTranscodeConsumer 无返回值的语义一致）。重复调用是幂等的（原为 sync.Once）。
func (c *TranscodeConsumerComponent) Start(_ context.Context) error {
	c.mu.Lock()

	if c.started {
		c.mu.Unlock()

		return nil
	}

	c.started = true

	if c.cfg == nil || !c.cfg.Enable {
		c.mu.Unlock()
		c.logger.Info("kafka disabled, skip transcode consumer")

		return nil
	}

	// 重试循环独立于 Start 的 ctx：原实现用 context.Background()，由 Stop 显式取消。
	retryCtx, cancel := context.WithCancel(context.Background())
	c.retryCancel = cancel
	c.mu.Unlock()

	go c.startWithRetry(retryCtx)

	return nil
}

// Stop 取消进行中的初始化重试，并优雅关闭已启动的消费者。
func (c *TranscodeConsumerComponent) Stop(_ context.Context) error {
	c.mu.Lock()
	cancel := c.retryCancel
	consumer := c.consumer
	c.retryCancel = nil
	c.consumer = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if consumer == nil {
		return nil
	}

	if err := consumer.Stop(); err != nil {
		c.logger.Error("stop transcode consumer failed", zap.Error(err))
	}

	return nil
}

// startWithRetry 反复尝试构造并启动消费者，直到成功、永久失败、超出总窗口或被取消。
func (c *TranscodeConsumerComponent) startWithRetry(ctx context.Context) {
	startedAt := time.Now()
	backoff := transcodeConsumerInitialBackoff

	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			c.logger.Info("transcode consumer init canceled")

			return
		}

		consumer, err := NewTranscodeConsumer(
			c.cfg,
			c.service,
			c.registry,
			c.subscriber,
			c.producers,
			c.logger,
		)
		if err == nil {
			if startErr := consumer.Start(); startErr != nil {
				_ = consumer.Stop()

				err = startErr
			} else {
				if !c.install(ctx, consumer) {
					return
				}

				c.logger.Info("transcode consumer ready",
					zap.Int("attempts", attempt),
					zap.Duration("cost", time.Since(startedAt)),
				)

				return
			}
		}

		if !isRetryableTranscodeInitError(err) {
			c.logger.Error("init transcode consumer failed permanently",
				zap.Error(err),
				zap.Int("attempt", attempt),
			)

			return
		}

		elapsed := time.Since(startedAt)
		if elapsed >= transcodeConsumerMaxWait {
			c.logger.Error("init transcode consumer gave up after max wait",
				zap.Error(err),
				zap.Int("attempts", attempt),
				zap.Duration("elapsed", elapsed),
				zap.Duration("maxWait", transcodeConsumerMaxWait),
			)

			return
		}

		sleep := backoff
		if remaining := transcodeConsumerMaxWait - elapsed; sleep > remaining {
			sleep = remaining
		}

		c.logger.Warn("init transcode consumer failed, will retry",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Duration("nextRetryIn", sleep),
			zap.Duration("elapsed", elapsed),
		)

		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			c.logger.Info("transcode consumer init canceled")

			return
		case <-timer.C:
		}

		backoff = nextTranscodeInitBackoff(backoff)
	}
}

// install 在未取消时登记消费者；若已取消则立刻 Stop 并返回 false。
func (c *TranscodeConsumerComponent) install(ctx context.Context, consumer *TranscodeConsumer) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ctx.Err() != nil {
		_ = consumer.Stop()
		c.logger.Info("transcode consumer init canceled after start")

		return false
	}

	c.consumer = consumer

	return true
}

// nextTranscodeInitBackoff 计算下一次退避：2s → 4s → 8s → 16s → 30s（封顶）。
func nextTranscodeInitBackoff(current time.Duration) time.Duration {
	if current <= 0 {
		return transcodeConsumerInitialBackoff
	}

	if current >= transcodeConsumerMaxBackoff {
		return transcodeConsumerMaxBackoff
	}

	next := current * 2
	if next > transcodeConsumerMaxBackoff || next < current {
		return transcodeConsumerMaxBackoff
	}

	return next
}

// isRetryableTranscodeInitError 区分配置类永久错误与连接类可重试错误。
//
// 永久错误关键字列表逐字保留原实现（新代码的构造函数错误文案必须与之匹配）。
func isRetryableTranscodeInitError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	for _, permanent := range []string{
		"application config not initialized",
		"kafka not enabled",
		"transcode config not configured",
		"transcode topic not configured",
		"kafka brokers not configured",
		"kafka groupid required",
		"topics list required",
		"message handler required",
	} {
		if strings.Contains(msg, permanent) {
			return false
		}
	}

	// 默认按可重试处理：覆盖 out of brokers / EOF / connection refused / timeout 等。
	return true
}
