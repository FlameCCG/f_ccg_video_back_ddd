// Package wslog records message outcomes at the WebSocket boundary.
package wslog

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gorilla/websocket"

	"go.uber.org/zap"
)

// Connection adds a server-owned connection identity.
func Connection(ctx context.Context, userID string) context.Context {
	v := logmeta.From(ctx)
	v.ConnectionID = logmeta.NewID()
	v.UserID = userID

	return logmeta.With(ctx, v)
}

// Message adds an independent identity for one inbound message.
func Message(ctx context.Context) context.Context {
	v := logmeta.From(ctx)
	v.MessageID = logmeta.NewID()

	return logmeta.With(ctx, v)
}

// Run contains message panics and classifies each result once, without logging message content.
func Run(ctx context.Context, logger *zap.Logger, operation string, fn func(context.Context) error) {
	ctx = Message(ctx)
	start := time.Now()

	var (
		err   error
		stack string
	)

	defer func() {
		if value := recover(); value != nil {
			err = apperr.Wrap(operation, nil, fmt.Errorf("panic: %v", value), apperr.Internal)
			stack = string(debug.Stack())
		}

		Result(ctx, logger, operation, err, stack, time.Since(start))
	}()

	err = fn(ctx)
}

// Result records a message summary and any required boundary exception.
func Result(ctx context.Context, logger *zap.Logger, operation string, err error, stack string, duration time.Duration) {
	logger = logctx.With(ctx, logger).With(zap.String("component", "websocket"))

	fields := []zap.Field{zap.String("event", "websocket.message"), zap.String("operation", operation), zap.Bool("success", err == nil), zap.Float64("duration_ms", float64(duration)/float64(time.Millisecond))}
	if err != nil {
		d := clienterror.Describe(err)
		if ctx != nil && errors.Is(ctx.Err(), context.Canceled) && errors.Is(err, context.Canceled) {
			d.Kind, d.Code = apperr.Canceled, "websocket.canceled"
		}

		fields = append(fields, zap.String("error_code", d.Code), zap.String("error_kind", string(d.Kind)))

		detail := []zap.Field{zap.String("event", "websocket.message.failed"), zap.String("operation", operation), zap.String("error_kind", string(d.Kind)), zap.String("error_code", d.Code), zap.String("error", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))}
		if stack != "" {
			detail = append(detail, zap.String("stack", logsafe.Text(stack, 0)))
		}

		switch d.Kind {
		case apperr.Validation, apperr.Rejected, apperr.NotFound, apperr.Conflict, apperr.RateLimited:
		case apperr.Canceled:
			logger.Info("websocket.message.canceled", detail...)
		case apperr.Security, apperr.Degraded:
			logger.Warn("websocket.message.failed", detail...)
		case apperr.Dependency, apperr.Internal:
			fallthrough
		default:
			logger.Error("websocket.message.failed", detail...)
		}
	}

	logger.Info("websocket.message", fields...)
}

// UpgradeFailure distinguishes protocol refusals and origin violations from server failures.
func UpgradeFailure(err error, status int) error {
	kind := apperr.Internal

	var handshake websocket.HandshakeError
	if errors.As(err, &handshake) {
		kind = apperr.Rejected
		if status == http.StatusForbidden {
			kind = apperr.Security
		}
	}

	return apperr.Wrap("websocket.upgrade", nil, err, kind)
}
