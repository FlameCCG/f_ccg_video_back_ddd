// Package logctx enriches explicitly injected Zap loggers with context metadata.
package logctx

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logmeta"

	"go.uber.org/zap"
)

// With enriches an explicitly injected logger with non-empty correlation values.
func With(ctx context.Context, logger *zap.Logger) *zap.Logger {
	if logger == nil {
		logger = zap.NewNop()
	}

	v := logmeta.From(ctx)

	var fields []zap.Field

	for _, pair := range [][2]string{{"request_id", v.RequestID}, {"user_id", v.UserID}, {"task_id", v.TaskID}, {"parent_task_id", v.ParentTaskID}, {"attempt_id", v.AttemptID}, {"connection_id", v.ConnectionID}, {"message_id", v.MessageID}} {
		if pair[1] != "" {
			fields = append(fields, zap.String(pair[0], pair[1]))
		}
	}

	return logger.With(fields...)
}
