package mq

import (
	"context"
	"crypto/sha256"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"fmt"

	"github.com/IBM/sarama"
)

const (
	ffmpegHideBanner = "-hide_banner"
	testStableTask   = "stable-task"
)

// Header correlation is additive; existing task JSON and legacy producers remain compatible.
func correlationHeaders(v logmeta.Values) []sarama.RecordHeader {
	var headers []sarama.RecordHeader

	for _, pair := range [][2]string{{"request_id", v.RequestID}, {"user_id", v.UserID}, {"task_id", v.TaskID}, {"parent_task_id", v.ParentTaskID}, {"attempt_id", v.AttemptID}} {
		if pair[1] != "" {
			headers = append(headers, sarama.RecordHeader{Key: []byte(pair[0]), Value: []byte(pair[1])})
		}
	}

	return headers
}

func messageContext(parent context.Context, msg *sarama.ConsumerMessage) context.Context {
	v := logmeta.From(parent)

	for _, header := range msg.Headers {
		if header == nil || !validCorrelation(string(header.Value)) {
			continue
		}

		value := string(header.Value)
		switch string(header.Key) {
		case "request_id":
			v.RequestID = value
		case "user_id":
			v.UserID = value
		case "task_id":
			v.TaskID = value
		case "parent_task_id":
			v.ParentTaskID = value
		case "attempt_id":
			v.AttemptID = value
		}
	}

	if v.TaskID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", msg.Topic, msg.Partition, msg.Offset)))
		v.TaskID = fmt.Sprintf("legacy-%x", sum[:16])
	}

	return logmeta.With(parent, v)
}

func validCorrelation(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}

	for _, r := range value {
		if validCorrelationCondition(r) {
			return false
		}
	}

	return true
}

func setCorrelation(msg *sarama.ConsumerMessage, v logmeta.Values) {
	out := make([]*sarama.RecordHeader, 0, len(msg.Headers)+4)
	for _, header := range msg.Headers {
		if header == nil {
			continue
		}

		switch string(header.Key) {
		case "request_id", "user_id", "task_id", "parent_task_id", "attempt_id":
			continue
		}

		out = append(out, header)
	}

	for _, header := range correlationHeaders(v) {
		h := header
		out = append(out, &h)
	}

	msg.Headers = out
}

type correlatedSession struct {
	sarama.ConsumerGroupSession
	ctx context.Context //nolint:containedctx // Decorates a ConsumerGroupSession with the same session lifetime and correlation metadata.
}

func (s correlatedSession) Context() context.Context { return s.ctx }

func validCorrelationCondition(r rune) bool {
	return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' && r != ':'
}
