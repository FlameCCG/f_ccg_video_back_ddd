// Package mq 提供 Kafka 驱动、消息契约与转码消费能力（原 pkg/mq + service/mq_service）。
//
// 包内不存在包级可变状态：生产者缓存、worker 计数、GPU 冷却等原包级变量
// 全部改为结构体字段并由构造函数注入。
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
// 原实现由 NewMQService() 在每个投递点重新构造（Kafka 未启用时构造即失败），
// 新架构改为单实例注入 + 投递时惰性解析驱动，错误文案与时机与原实现一致。
type MQService struct {
	cfg       *config.Kafka
	producers *ProducerPool
	logger    *zap.Logger
}

// 编译期断言：满足领域侧任务投递端口。
var _ media.TaskPublisher = (*MQService)(nil)

// NewMQService 构造 MQ 服务。
func NewMQService(cfg *config.Kafka, producers *ProducerPool, logger *zap.Logger) *MQService {
	return &MQService{cfg: cfg, producers: producers, logger: logger}
}

// resolve 解析驱动与业务主题表，错误顺序与原 NewMQService 完全一致。
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

// PublishRaw 直接投递已序列化的转码消息（保留原 PublishTranscodeTask([]byte) 能力）。
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
