// Package mq 实现 Kafka 传输和 FFmpeg 转码执行适配器。
//
// 先读 payload.go 与 kafka_driver.go 的消息契约，再读 kafka_consumer.go 的确认、重试和死信分支。
// transcode_consumer*.go 将消息接到 media 用例并限制任务并发，transcode_engine.go 等文件负责进程与产物。
// 领域计划描述要做什么，本包决定如何执行命令、记录检查点、上传并验证 DASH。
// 消息标记、实际提交 offset 与业务成功是不同阶段，进程中断或 rebalance 仍可能重投，恢复机制需容忍重复执行。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// TopicTranscode 是视频转码任务的逻辑 topic 名。
const TopicTranscode = "transcode"

// Driver 定义 MQ 行为。
type Driver interface {
	// Publish 向指定物理 topic 发送一条消息。
	Publish(ctx context.Context, topic string, payload []byte) error
}

// Service 对外暴露统一的 MQ 能力。
//
// 原实现由 NewMQService() 在每个投递点重新构造（Kafka 未启用时构造即失败），
// 新架构改为单实例注入 + 投递时惰性解析驱动，错误文案与时机与原实现一致。
type Service struct {
	cfg       *config.Kafka
	producers *ProducerPool
	logger    *zap.Logger
}

// 编译期断言：满足领域侧任务投递端口。
var _ media.TaskPublisher = (*Service)(nil)

// NewMQService 构造 MQ 服务。
func NewMQService(cfg *config.Kafka, producers *ProducerPool, logger *zap.Logger) *Service {
	return &Service{cfg: cfg, producers: producers, logger: logger}
}

// resolve 解析驱动与业务主题表，错误顺序与原 NewMQService 完全一致。
func (s *Service) resolve() (Driver, map[string]string, error) {
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
func (s *Service) Publish(ctx context.Context, topicAlias string, payload []byte) error {
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
func (s *Service) PublishTranscodeTask(ctx context.Context, task media.TranscodeTask) error {
	payload, err := json.Marshal(PayloadFromTask(task))
	if err != nil {
		return fmt.Errorf("marshal transcode payload: %w", err)
	}

	return s.Publish(ctx, TopicTranscode, payload)
}

// PublishRaw 直接投递已序列化的转码消息（保留原 PublishTranscodeTask([]byte) 能力）。
func (s *Service) PublishRaw(ctx context.Context, payload []byte) error {
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
