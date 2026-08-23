package mq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"f_ccg_video_back_ddd/internal/config"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// ProducerFactory 构造 sarama 同步生产者，便于测试替换。
type ProducerFactory func(brokers []string, cfg *sarama.Config) (sarama.SyncProducer, error)

// ProducerPool 缓存并复用 Kafka 同步生产者。
//
// 原实现使用包级变量 producerMu / sharedProducer / newSyncProducer；
// 新架构改为可注入结构体：初始化失败不缓存，便于 broker 恢复后重试（语义不变）。
type ProducerPool struct {
	mu      sync.Mutex
	shared  sarama.SyncProducer
	factory ProducerFactory
	logger  *zap.Logger
}

// NewProducerPool 创建生产者缓存（使用 sarama.NewSyncProducer）。
func NewProducerPool(logger *zap.Logger) *ProducerPool {
	return &ProducerPool{factory: sarama.NewSyncProducer, logger: logger}
}

// newProducerPoolWithFactory 用自定义工厂创建生产者缓存（测试用）。
func newProducerPoolWithFactory(factory ProducerFactory, logger *zap.Logger) *ProducerPool {
	return &ProducerPool{factory: factory, logger: logger}
}

// Producer 初始化（或复用）Kafka 同步生产者。
func (p *ProducerPool) Producer(cfg *config.Kafka) (sarama.SyncProducer, error) {
	if p == nil {
		return nil, errors.New("kafka producer pool not initialized")
	}

	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.shared != nil {
		return p.shared, nil
	}

	// 创建 Kafka 配置
	kCfg := sarama.NewConfig()
	kCfg.ClientID = cfg.ClientID
	kCfg.Producer.RequiredAcks = sarama.WaitForAll
	kCfg.Producer.Retry.Max = 5
	kCfg.Producer.Return.Successes = true
	kCfg.Producer.Return.Errors = true

	// 解析配置的 Kafka 版本
	if v := strings.TrimSpace(cfg.Version); v != "" {
		version, err := sarama.ParseKafkaVersion(v)
		if err != nil {
			p.logger.Warn("parse kafka version failed, use default", zap.Error(err))
		} else {
			kCfg.Version = version
		}
	}

	factory := p.factory
	if factory == nil {
		factory = sarama.NewSyncProducer
	}

	producer, err := factory(cfg.Brokers, kCfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer failed: %w", err)
	}

	p.shared = producer

	return p.shared, nil
}

// Close 关闭已建立的生产者连接（进程退出时由组合根调用）。
//
// 原实现从不关闭共享生产者（进程退出前一直泄漏），此处补齐资源回收；
// 调用时机由 bootstrap 掌握，未调用时行为与原实现完全一致。
func (p *ProducerPool) Close() error {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.shared == nil {
		return nil
	}

	producer := p.shared
	p.shared = nil

	return producer.Close()
}

// kafkaDriver 基于 sarama 同步生产者的 MQ 驱动。
type kafkaDriver struct {
	cfg      *config.Kafka
	producer sarama.SyncProducer
	logger   *zap.Logger
}

// NewKafkaDriver 创建 Kafka 驱动。
func NewKafkaDriver(cfg *config.Kafka, producers *ProducerPool, logger *zap.Logger) (Driver, error) {
	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers not configured")
	}

	if producers == nil {
		producers = NewProducerPool(logger)
	}

	producer, err := producers.Producer(cfg)
	if err != nil {
		return nil, err
	}

	return &kafkaDriver{
		cfg:      cfg,
		producer: producer,
		logger:   logger,
	}, nil
}

// Publish 同步发送一条消息到指定 topic。
func (k *kafkaDriver) Publish(_ context.Context, topic string, payload []byte) error {
	if strings.TrimSpace(topic) == "" {
		return errors.New("topic cannot be empty")
	}

	if len(payload) == 0 {
		return errors.New("payload cannot be empty")
	}

	if k.producer == nil {
		return errors.New("kafka producer not initialized")
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	}

	partition, offset, err := k.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("send kafka message failed: %w", err)
	}

	k.logger.Debug("kafka message sent",
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
		zap.String("topic", topic),
	)

	return nil
}
