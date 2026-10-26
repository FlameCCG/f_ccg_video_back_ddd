package logger

import (
	"bytes"
	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func fileConfig(t *testing.T) *config.Logger {
	t.Helper()
	dir := t.TempDir()

	return &config.Logger{DebugFileName: filepath.Join(dir, "debug.log"), InfoFileName: filepath.Join(dir, "info.log"), WarnFileName: filepath.Join(dir, "warn.log"), ErrorFileName: filepath.Join(dir, "error.log")}
}

func TestFileOutputsRetainSevereLevelsAndCloseSafely(t *testing.T) {
	t.Parallel()
	cfg := fileConfig(t)

	lg, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, level := range []zapcore.Level{zap.DebugLevel, zap.InfoLevel, zap.WarnLevel, zap.ErrorLevel, zap.DPanicLevel, zap.PanicLevel, zap.FatalLevel} {
		entry := zapcore.Entry{Level: level, Time: time.Now(), Message: "test severity"}
		if checked := lg.Zap().Core().Check(entry, nil); checked != nil {
			checked.Write()
		}
	}

	lg.Stop()
	lg.Stop()

	raw, err := os.ReadFile(cfg.ErrorFileName)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Count(string(raw), "test severity") != 4 {
		t.Fatalf("severe levels lost: %s", raw)
	}

	for _, level := range []string{"error", "dpanic", "panic", "fatal"} {
		if !strings.Contains(string(raw), `"level":"`+level+`"`) {
			t.Fatalf("missing %s: %s", level, raw)
		}
	}

	debug, err := os.ReadFile(cfg.DebugFileName)
	if err != nil {
		t.Fatal(err)
	}

	if len(debug) != 0 {
		t.Fatal("default Info level accepted Debug")
	}

	next, err := New(cfg)
	if err != nil {
		t.Fatalf("locks not released: %v", err)
	}

	next.Stop()
}

func TestFileValidationAndExclusiveOwnership(t *testing.T) {
	t.Parallel()
	cfg := fileConfig(t)

	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Stop()

	if second, err := New(cfg); err == nil {
		second.Stop()
		t.Fatal("shared rotation files accepted")
	}

	duplicate := *cfg

	duplicate.WarnFileName = duplicate.InfoFileName
	if lg, err := New(&duplicate); err == nil {
		lg.Stop()
		t.Fatal("duplicate level paths accepted")
	}

	broken := *cfg

	broken.DebugFileName = "/proc/logger-governance/unwritable.log"
	if lg, err := New(&broken); err == nil {
		lg.Stop()
		t.Fatal("unwritable output accepted")
	}

	if _, err := New(nil); err == nil {
		t.Fatal("nil config accepted")
	}
}

func TestDynamicLevelIsSharedAndConcurrent(t *testing.T) {
	t.Parallel()
	cfg := fileConfig(t)
	provider := infraconfig.NewProvider(&config.Config{Logger: cfg})

	lg, cleanup, err := Provide(cfg, provider)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	child := lg.Zap().With(zap.String("component", "child"))
	if child.Core().Enabled(zap.DebugLevel) {
		t.Fatal("Debug enabled by default")
	}

	changed := *cfg
	changed.Level = "debug"
	provider.Set(&config.Config{Logger: &changed})

	if !child.Core().Enabled(zap.DebugLevel) {
		t.Fatal("existing logger did not receive dynamic level")
	}

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := range 50 {
				child.Debug("progress")

				copyCfg := *cfg
				if i%2 == 0 {
					copyCfg.Level = "debug"
				}

				provider.Set(&config.Config{Logger: &copyCfg})
			}
		}()
	}

	wg.Wait()
}

func TestNewOutputModesAndLegacyDefaults(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"stdout", "files", "both", ""} {
		cfg := fileConfig(t)
		cfg.Output = mode

		lg, err := New(cfg)
		if err != nil {
			t.Fatal(err)
		}

		want := 4
		if mode == "stdout" {
			want = 0
		}

		if len(lg.writers) != want {
			t.Fatalf("mode %q writers=%d want=%d", mode, len(lg.writers), want)
		}

		lg.Stop()
	}
}

type recoverableWriter struct{ failing bool }

func (w *recoverableWriter) Write(p []byte) (int, error) {
	if w.failing {
		return 0, os.ErrPermission
	}

	return len(p), nil
}

func TestWriteFailureDiagnosesFirstAndRecoveryWithoutRecursion(t *testing.T) {
	t.Parallel()

	var diagnostics bytes.Buffer

	target := &recoverableWriter{failing: true}

	writer := &diagnosticWriter{target: target, diagnostics: &diagnostics}
	for range 3 {
		_, _ = writer.Write([]byte("terminal error"))
	}

	if strings.Count(diagnostics.String(), "logger.write.failed") != 1 {
		t.Fatalf("failure diagnostics not limited: %s", diagnostics.String())
	}

	target.failing = false
	_, _ = writer.Write([]byte("recovered"))

	if strings.Count(diagnostics.String(), "logger.write.recovered") != 1 {
		t.Fatal("recovery not visible")
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDebugSamplingNeverDropsTerminalOrSecurityEvents(t *testing.T) {
	t.Parallel()
	cfg := fileConfig(t)
	cfg.Level = "debug"

	lg, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for range 30 {
		lg.Zap().Debug("progress")
		lg.Zap().Error("terminal")
		lg.Zap().Warn("security")
	}

	lg.Stop()

	debug, _ := os.ReadFile(cfg.DebugFileName)
	failure, _ := os.ReadFile(cfg.ErrorFileName)

	security, _ := os.ReadFile(cfg.WarnFileName)
	if strings.Count(string(debug), "progress") >= 30 || strings.Count(string(failure), "terminal") != 30 || strings.Count(string(security), "security") != 30 {
		t.Fatal("sampling dropped a terminal/security event or failed to limit progress")
	}
}
