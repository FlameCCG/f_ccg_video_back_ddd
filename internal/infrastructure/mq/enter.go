// Package mq 提供 Kafka 驱动、消息契约与转码消费能力。
//
// 生产者缓存、worker 计数和 GPU 冷却由实例字段持有，并通过构造函数注入依赖。
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// TopicTranscode 是视频转码任务的逻辑 topic 名。
const TopicTranscode = "transcode"

// Driver 定义 MQ 行为。
type Driver interface {
	// Publish 向指定物理 topic 发送一条消息。
	Publish(ctx context.Context, topic string, payload []byte) error
}

// MQService 对外暴露统一的 MQ 能力。
//
// 服务在构造时注入，投递消息时解析 Kafka 驱动与业务主题配置；未启用 Kafka 时返回配置错误。
type MQService struct {
	cfg       *config.Kafka
	producers *ProducerPool
	logger    *zap.Logger
}

// 编译期断言：满足领域侧任务投递端口。
var _ media.TaskPublisher = (*MQService)(nil)

// NewMQService 构造 MQ 服务。
func NewMQService(cfg *config.Kafka, producers *ProducerPool, logger *zap.Logger) *MQService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MQService{cfg: cfg, producers: producers, logger: logger}
}

// resolve 依次校验驱动与业务主题配置，并返回相应错误。
func (s *MQService) resolve() (Driver, map[string]string, error) {
	if s == nil {
		return nil, nil, errors.New("mq service disabled")
	}

	cfg := s.cfg
	if cfg == nil || !cfg.Enable {
		return nil, nil, errors.New("mq service disabled")
	}

	driver, err := NewKafkaDriver(cfg, s.producers, s.logger)
	if err != nil {
		return nil, nil, err
	}

	topics := sanitizeTopics(cfg.Topics)
	if len(topics) == 0 {
		return nil, nil, errors.New("no kafka topics configured")
	}

	return driver, topics, nil
}

// Publish 发布到指定业务主题。
func (s *MQService) Publish(ctx context.Context, topicAlias string, payload []byte) error {
	driver, topics, err := s.resolve()
	if err != nil {
		return err
	}

	if driver == nil {
		return errors.New("mq driver not initialized")
	}

	topic, ok := topics[topicAlias]
	if !ok || strings.TrimSpace(topic) == "" {
		return fmt.Errorf("topic %s not configured", topicAlias)
	}

	return driver.Publish(ctx, topic, payload)
}

// PublishTranscodeTask 把任务序列化为 Kafka 消息并发送到异步转码队列。
func (s *MQService) PublishTranscodeTask(ctx context.Context, task media.TranscodeTask) error {
	payload, err := json.Marshal(PayloadFromTask(task))
	if err != nil {
		return fmt.Errorf("marshal transcode payload: %w", err)
	}

	return s.Publish(ctx, TopicTranscode, payload)
}

// PublishRaw 直接投递已序列化的转码消息。
func (s *MQService) PublishRaw(ctx context.Context, payload []byte) error {
	return s.Publish(ctx, TopicTranscode, payload)
}

func sanitizeTopics(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))

	for key, topic := range m {
		if t := strings.TrimSpace(topic); t != "" {
			result[key] = t
		}
	}

	return result
}
