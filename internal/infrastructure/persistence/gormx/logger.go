package gormx

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/logctx"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ZapLogger deliberately never evaluates SQL text. Interpolated parameters cannot escape via Trace.
// SQL errors are diagnostics at Warn; the business processing boundary owns Error alerts.
type ZapLogger struct {
	Logger        *zap.Logger
	SlowThreshold time.Duration
	Level         logger.LogLevel
}

// NewZapLogger creates a parameter-free diagnostic adapter with a 200ms default threshold.
func NewZapLogger(lg *zap.Logger, threshold time.Duration) logger.Interface {
	if threshold <= 0 {
		threshold = 200 * time.Millisecond
	}

	return &ZapLogger{Logger: lg, SlowThreshold: threshold, Level: logger.Warn}
}

// LogMode returns an independent level view of the same injected logger.
func (l *ZapLogger) LogMode(level logger.LogLevel) logger.Interface {
	next := *l
	next.Level = level

	return &next
}

// Info records library diagnostics without interpolating arbitrary SQL arguments.
func (l *ZapLogger) Info(ctx context.Context, _ string, _ ...any) {
	if l.Level >= logger.Info {
		logctx.With(ctx, l.Logger).Debug("db.diagnostic", zap.String("event", "db.diagnostic"))
	}
}

// Warn records library diagnostics without interpolating arbitrary SQL arguments.
func (l *ZapLogger) Warn(ctx context.Context, _ string, _ ...any) {
	if l.Level >= logger.Warn {
		logctx.With(ctx, l.Logger).Warn("db.diagnostic", zap.String("event", "db.diagnostic"))
	}
}

func (l *ZapLogger) Error(ctx context.Context, _ string, _ ...any) {
	if l.Level >= logger.Error {
		logctx.With(ctx, l.Logger).Warn("db.diagnostic.failed", zap.String("event", "db.diagnostic.failed"))
	}
}

// Trace records slow/error diagnostics without invoking the SQL formatter.
func (l *ZapLogger) Trace(ctx context.Context, begin time.Time, _ func() (string, int64), err error) {
	if l.Level == logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	fields := []zap.Field{zap.String("component", "database"), zap.Float64("duration_ms", float64(elapsed)/float64(time.Millisecond))}
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && l.Level >= logger.Error:
		// The original error is retained by GORM for the caller; diagnostic logs use no driver message/parameters.
		fields = append(fields, zap.String("event", "db.query.failed"))
		logctx.With(ctx, l.Logger).Warn("db.query.failed", fields...)
	case elapsed >= l.SlowThreshold && l.Level >= logger.Warn:
		fields = append(fields, zap.String("event", "db.query.slow"))
		logctx.With(ctx, l.Logger).Warn("db.query.slow", fields...)
	}
}

// ParamsFilter also prevents parameter expansion in GORM's ExplainSQL paths.
func (*ZapLogger) ParamsFilter(_ context.Context, sql string, _ ...any) (string, []any) {
	return sql, nil
}
