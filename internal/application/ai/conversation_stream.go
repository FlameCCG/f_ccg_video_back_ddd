package ai

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// responseStream 持有用例 context，不能在 OpenResponseStream 返回时提前取消。
// Recv 与 Eino 一样由单个消费者调用，Close 可安全地重复调用。
type responseStream struct {
	source    *schema.StreamReader[*schema.Message]
	cancel    context.CancelFunc
	closeOnce sync.Once
	start     time.Time
	first     bool
	metadata  ResponseMetadata
	logger    *zap.Logger
}

func (s *responseStream) Recv() (*schema.Message, error) {
	message, err := s.source.Recv()
	s.metadata.Observe(message)
	if !s.first && message != nil && (message.Content != "" || message.ReasoningContent != "") {
		s.first = true
		s.logger.Debug("AI response first chunk", zap.Duration("elapsed", time.Since(s.start)))
	}
	if err != nil {
		s.cancel()
		fields := responseLogFields(s.metadata)
		fields = append(fields, zap.Duration("elapsed", time.Since(s.start)))
		if !errors.Is(err, io.EOF) {
			fields = append(fields, zap.Error(err))
		}
		s.logger.Debug("AI response stream ended", fields...)
	}
	return message, err
}

func (s *responseStream) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		s.source.Close()
	})
}

func (s *Service) invokeText(ctx context.Context, invocation TextInvocation) (TextResult, error) {
	started := time.Now()
	result, err := s.text.InvokeText(ctx, invocation)
	fields := responseLogFields(result.ResponseMetadata)
	fields = append(fields, zap.String("model", invocation.Model), zap.Duration("elapsed", time.Since(started)))
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	s.log().Debug("AI text invocation ended", fields...)
	return result, err
}

func responseLogFields(metadata ResponseMetadata) []zap.Field {
	fields := []zap.Field{zap.String("finishReason", metadata.FinishReason), zap.String("status", metadata.Status())}
	if usage := metadata.Usage; usage != nil {
		fields = append(fields, zap.Int("inputTokens", usage.InputTokens), zap.Int("outputTokens", usage.OutputTokens))
	}
	return fields
}

func (s *Service) log() *zap.Logger {
	if s.logger == nil {
		return zap.NewNop()
	}
	return s.logger
}
