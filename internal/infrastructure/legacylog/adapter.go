// Package legacylog adapts the third-party canal global API at process composition time.
// Application code must never import the legacy logger.
package legacylog

import (
	"strings"
	"sync"
	"sync/atomic"

	legacy "github.com/siddontang/go-log/log"
	"go.uber.org/zap"
)

var (
	installOnce sync.Once
	target      atomic.Pointer[zap.Logger]
)

// Attach is the only process-global logger bridge, required by the legacy canal API.
// The immutable bridge reads its explicitly attached destination atomically.
func Attach(logger *zap.Logger) func() {
	target.Store(logger)
	installOnce.Do(func() {
		bridge := legacy.New(handler{}, legacy.Llevel)
		bridge.SetLevel(legacy.LevelInfo)
		legacy.SetDefaultLogger(bridge)
	})

	return func() { target.CompareAndSwap(logger, nil) }
}

type handler struct{}

func (handler) Close() error { return nil }
func (handler) Write(p []byte) (int, error) {
	logger := target.Load()
	if logger == nil {
		return len(p), nil
	}

	text := strings.TrimSpace(string(p))
	fields := []zap.Field{zap.String("component", "river_dependency"), zap.String("event", "river.dependency.diagnostic")}
	// Only the adapter's known level prefix is interpreted, never error-message strings.
	switch {
	case strings.HasPrefix(text, "[error]"), strings.HasPrefix(text, "[fatal]"):
		logger.Error(text, fields...)
	case strings.HasPrefix(text, "[warn]"):
		logger.Warn(text, fields...)
	case strings.HasPrefix(text, "[debug]"), strings.HasPrefix(text, "[trace]"):
		logger.Debug(text, fields...)
	default:
		logger.Info(text, fields...)
	}

	return len(p), nil
}
