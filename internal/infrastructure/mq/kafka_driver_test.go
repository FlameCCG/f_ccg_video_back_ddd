package mq

import (
	"errors"
	"testing"

	"f_ccg_video_back_ddd/internal/config"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// TestProducerPoolRetriesAfterFailure 初始化失败不缓存，broker 恢复后可重试成功
// （原 TestInitKafkaProducerRetriesAfterFailure：包级 producerMu/sharedProducer/newSyncProducer
// 改为可注入的 *ProducerPool + ProducerFactory）。
func TestProducerPoolRetriesAfterFailure(t *testing.T) {
	t.Parallel()

	expectedProducer := stubSyncProducer{}

	var calls int

	pool := newProducerPoolWithFactory(func([]string, *sarama.Config) (sarama.SyncProducer, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("temporary kafka error")
		}

		return expectedProducer, nil
	}, zap.NewNop())

	cfg := &config.Kafka{Brokers: []string{"127.0.0.1:9092"}}
	if _, err := pool.Producer(cfg); err == nil {
		t.Fatal("expected first producer initialization to fail")
	}

	producer, err := pool.Producer(cfg)
	if err != nil {
		t.Fatalf("expected producer initialization retry to succeed: %v", err)
	}

	if producer != expectedProducer {
		t.Fatalf("expected cached producer %T, got %T", expectedProducer, producer)
	}

	// 第三次调用应复用缓存，不再触碰工厂。
	if _, err := pool.Producer(cfg); err != nil {
		t.Fatalf("cached producer lookup failed: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected producer factory to be called twice, got %d", calls)
	}
}

// TestProducerPoolRejectsMissingBrokers 未配置 broker 时报错文案属于消费者重试分类的输入。
func TestProducerPoolRejectsMissingBrokers(t *testing.T) {
	t.Parallel()

	pool := NewProducerPool(zap.NewNop())

	if _, err := pool.Producer(nil); err == nil {
		t.Fatal("nil config must be rejected")
	}

	_, err := pool.Producer(&config.Kafka{})
	if err == nil {
		t.Fatal("empty brokers must be rejected")
	}

	if isRetryableTranscodeInitError(err) {
		t.Fatalf("kafka brokers not configured must be permanent: %v", err)
	}
}

// TestProducerPoolCloseReleasesProducer Close 后可重新建立连接（原实现从不关闭，此处补齐资源回收）。
func TestProducerPoolCloseReleasesProducer(t *testing.T) {
	t.Parallel()

	var calls int

	pool := newProducerPoolWithFactory(func([]string, *sarama.Config) (sarama.SyncProducer, error) {
		calls++

		return stubSyncProducer{}, nil
	}, zap.NewNop())

	cfg := &config.Kafka{Brokers: []string{"127.0.0.1:9092"}}
	if _, err := pool.Producer(cfg); err != nil {
		t.Fatalf("first producer: %v", err)
	}

	if err := pool.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Close 幂等。
	if err := pool.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	if _, err := pool.Producer(cfg); err != nil {
		t.Fatalf("producer after close: %v", err)
	}

	if calls != 2 {
		t.Fatalf("factory calls = %d, want 2 (rebuilt after Close)", calls)
	}
}

// TestKafkaDriverPublishValidatesInput Publish 的入参校验与错误文案。
func TestKafkaDriverPublishValidatesInput(t *testing.T) {
	t.Parallel()

	pool := newProducerPoolWithFactory(func([]string, *sarama.Config) (sarama.SyncProducer, error) {
		return stubSyncProducer{}, nil
	}, zap.NewNop())

	driver, err := NewKafkaDriver(
		&config.Kafka{Brokers: []string{"127.0.0.1:9092"}},
		pool,
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("NewKafkaDriver() error = %v", err)
	}

	if err := driver.Publish(t.Context(), "", []byte("x")); err == nil {
		t.Fatal("empty topic must be rejected")
	}

	if err := driver.Publish(t.Context(), "transcode", nil); err == nil {
		t.Fatal("empty payload must be rejected")
	}

	if err := driver.Publish(t.Context(), "transcode", []byte("x")); err != nil {
		t.Fatalf("valid publish failed: %v", err)
	}
}

// TestMQServiceResolveErrorOrder 投递前的错误顺序与原 NewMQService 完全一致。
func TestMQServiceResolveErrorOrder(t *testing.T) {
	t.Parallel()

	// Kafka 未启用。
	disabled := NewMQService(
		&config.Kafka{Enable: false},
		NewProducerPool(zap.NewNop()),
		zap.NewNop(),
	)
	if err := disabled.PublishRaw(t.Context(), []byte("x")); err == nil ||
		err.Error() != "mq service disabled" {
		t.Fatalf("disabled kafka error = %v, want \"mq service disabled\"", err)
	}

	// nil service 也走同一分支。
	var nilService *MQService
	if err := nilService.PublishRaw(t.Context(), []byte("x")); err == nil ||
		err.Error() != "mq service disabled" {
		t.Fatalf("nil service error = %v", err)
	}

	pool := newProducerPoolWithFactory(func([]string, *sarama.Config) (sarama.SyncProducer, error) {
		return stubSyncProducer{}, nil
	}, zap.NewNop())

	// 无 topic 配置。
	noTopics := NewMQService(
		&config.Kafka{Enable: true, Brokers: []string{"b"}},
		pool,
		zap.NewNop(),
	)
	if err := noTopics.PublishRaw(t.Context(), []byte("x")); err == nil ||
		err.Error() != "no kafka topics configured" {
		t.Fatalf("no topics error = %v", err)
	}

	// topic 别名未配置。
	otherTopic := NewMQService(&config.Kafka{
		Enable:  true,
		Brokers: []string{"b"},
		Topics:  map[string]string{"other": "other-topic"},
	}, pool, zap.NewNop())
	if err := otherTopic.PublishRaw(t.Context(), []byte("x")); err == nil {
		t.Fatal("missing transcode topic alias must be rejected")
	}
}

type stubSyncProducer struct{}

func (stubSyncProducer) SendMessage(*sarama.ProducerMessage) (int32, int64, error) {
	return 0, 0, nil
}

func (stubSyncProducer) SendMessages([]*sarama.ProducerMessage) error {
	return nil
}

func (stubSyncProducer) Close() error {
	return nil
}

func (stubSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	return 0
}

func (stubSyncProducer) IsTransactional() bool {
	return false
}

func (stubSyncProducer) BeginTxn() error {
	return nil
}

func (stubSyncProducer) CommitTxn() error {
	return nil
}

func (stubSyncProducer) AbortTxn() error {
	return nil
}

func (stubSyncProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}

func (stubSyncProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
