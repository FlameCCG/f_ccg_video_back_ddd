package middlewares

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"f_ccg_video_back_ddd/pkg/res"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestContext always issues a new server-owned ID, even if a client supplies one.
func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		values := logmeta.From(c.Request.Context())
		values.RequestID = logmeta.NewID()
		c.Request = c.Request.WithContext(logmeta.With(c.Request.Context(), values))
		c.Header("X-Request-ID", values.RequestID)
		c.Next()
	}
}

// GinLogger writes one access summary, plus one classified boundary detail when needed.
// It never logs queries, headers or bodies and never writes a response.
func GinLogger(lg *zap.Logger) gin.HandlerFunc {
	lg = loggerOrNop(lg)

	return func(c *gin.Context) {
		start := time.Now()
		requestContext := c.Request.Context()
		c.Next()
		logger := logctx.With(c.Request.Context(), lg).With(zap.String("component", "http"))

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		fields := []zap.Field{zap.String("event", "http.request"), zap.String("operation", c.Request.Method+" "+route), zap.String("route", route), zap.String("method", c.Request.Method), zap.Int("http_status", c.Writer.Status()), zap.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond))}
		if success, ok := res.Success(c); ok {
			fields = append(fields, zap.Bool("success", success))
		}

		err := res.Failure(c)
		if err == nil && len(c.Errors) > 0 {
			err = c.Errors.Last().Err
		}

		if err == nil && c.Writer.Status() >= http.StatusInternalServerError {
			err = errors.New("unhandled HTTP server failure")
		}

		if err != nil {
			fields = append(fields, logRequestFailure(requestContext, c, logger, err, route)...)
		}

		logger.Info("http.request", fields...)
	}
}

func brokenConnection(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && (errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET))
}

// GinRecovery records the panic without dumping the request or logging a second time.
// Broken connections retain Gin's abort-only behavior; other panics receive HTTP 500.
func GinRecovery(_ *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if value := recover(); value != nil {
				err, ok := value.(error)
				if !ok {
					err = fmt.Errorf("panic: %v", value)
				}

				panicStack := ""
				if stack {
					panicStack = string(debug.Stack())
				}

				res.RecordPanic(apperr.Wrap("http.panic", nil, err, apperr.Internal), panicStack, c)

				if brokenConnection(err) {
					c.Abort()
					return
				}

				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}

// RequestLogger returns the request summary and exception boundary middleware.
func (m *Middlewares) RequestLogger() gin.HandlerFunc { return GinLogger(m.log()) }

// Recovery returns the panic recovery middleware with safe error registration.
func (m *Middlewares) Recovery() gin.HandlerFunc { return GinRecovery(m.log(), true) }

func ginLoggerCondition(requestContext context.Context, err error) bool {
	return errors.Is(requestContext.Err(), context.Canceled) && (errors.Is(err, context.Canceled) || brokenConnection(err))
}

func logRequestFailure(requestContext context.Context, c *gin.Context, logger *zap.Logger, err error, route string) []zap.Field {
	descriptor := clienterror.Describe(err)
	if ginLoggerCondition(requestContext, err) {
		descriptor.Kind, descriptor.Code = apperr.Canceled, "request.canceled"
	} else if descriptor.Kind == apperr.Canceled {
		descriptor.Kind, descriptor.Code = apperr.Internal, "request.cancellation_unconfirmed"
	}

	fields := []zap.Field{zap.String("error_code", descriptor.Code), zap.String("error_kind", string(descriptor.Kind))}

	detail := []zap.Field{zap.String("event", "http.request.failed"), zap.String("operation", c.Request.Method+" "+route), zap.String("error_code", descriptor.Code), zap.String("error_kind", string(descriptor.Kind)), zap.String("error", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))}
	if stack := res.Stack(c); stack != "" {
		detail = append(detail, zap.String("stack", logsafe.Text(stack, 0)))
	}

	switch descriptor.Kind {
	case apperr.Validation, apperr.Rejected, apperr.NotFound, apperr.Conflict, apperr.RateLimited:
	case apperr.Canceled:
		logger.Info("http.request.canceled", detail...)
	case apperr.Security, apperr.Degraded:
		logger.Warn("http.request.failed", detail...)
	case apperr.Dependency, apperr.Internal:
		fallthrough
	default:
		logger.Error("http.request.failed", detail...)
	}

	return fields
}
