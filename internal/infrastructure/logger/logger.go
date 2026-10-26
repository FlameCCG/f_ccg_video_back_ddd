// Package logger constructs synchronous Zap outputs and owns rotation writers.
package logger

import (
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/infrastructure/legacylog"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger owns one synchronous output tree, its atomic level and rotation resources.
type Logger struct {
	lg           *zap.Logger
	level        zap.AtomicLevel
	writers      []*diagnosticWriter
	console      *diagnosticWriter
	locks        []*os.File
	stopOnce     sync.Once
	unsubscribe  func()
	detachLegacy func()
	redactor     *logsafe.Redactor
}

// Zap returns the explicitly owned logger.
func (l *Logger) Zap() *zap.Logger {
	if l == nil || l.lg == nil {
		return zap.NewNop()
	}

	return l.lg
}

// SetLevel updates every derived logger through the shared atomic level.
func (l *Logger) SetLevel(level string) error { return l.level.UnmarshalText([]byte(level)) }

// New validates every output eagerly, including locks independent of rotating file inodes.
func New(cfg *config.Logger) (*Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	l := &Logger{level: zap.NewAtomicLevel()}
	if err := l.SetLevel(cfg.EffectiveLevel()); err != nil {
		return nil, err
	}

	service, env, instance := cfg.Service, cfg.Env, cfg.Instance
	if service == "" {
		service = "video-backend"
	}

	if env == "" {
		env = "production"
		if cfg.Development {
			env = "development"
		}
	}

	if instance == "" {
		host, _ := os.Hostname()
		instance = host + "-" + strconv.Itoa(os.Getpid())
	}

	if slices.ContainsFunc([]string{service, env, instance}, newCondition2) {
		return nil, errors.New("logger identity must not contain path separators")
	}

	return finishNew(cfg, l, service, env, instance)
}

func encoder(format string) zapcore.Encoder {
	c := zap.NewProductionEncoderConfig()
	c.TimeKey = "time"
	c.EncodeTime = func(t time.Time, arr zapcore.PrimitiveArrayEncoder) {
		arr.AppendString(t.UTC().Format(time.RFC3339Nano))
	}

	c.EncodeDuration = zapcore.MillisDurationEncoder
	if format == "console" {
		return zapcore.NewConsoleEncoder(c)
	}

	return zapcore.NewJSONEncoder(c)
}

// Stop first removes subscriptions, then explicitly closes Lumberjack. Sync alone is insufficient.
func (l *Logger) Stop() {
	if l == nil {
		return
	}

	l.stopOnce.Do(func() {
		if l.detachLegacy != nil {
			l.detachLegacy()
		}

		if l.unsubscribe != nil {
			l.unsubscribe()
		}

		if l.lg != nil {
			_ = l.lg.Sync()
		}

		for _, writer := range l.writers {
			if err := writer.Close(); err != nil {
				fmt.Fprintln(os.Stderr, "logger.close.failed:", logsafe.Text(err.Error(), logsafe.ErrorLimit))
			}
		}

		if l.console != nil {
			_ = l.console.Close()
		}

		for _, lock := range l.locks {
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
			_ = lock.Close()
		}
	})
}

// diagnosticWriter reports failures independently of Zap and rate limits only repeated failures.
type diagnosticWriter struct {
	diagnostics io.Writer
	mu          sync.Mutex
	writer      *lumberjack.Logger
	target      io.Writer
	closed      bool
	failing     bool
	lastFailure time.Time
}

func (w *diagnosticWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}

	var (
		n   int
		err error
	)
	if w.target != nil {
		n, err = w.target.Write(p)
	} else {
		n, err = w.writer.Write(p)
	}

	diagnostics := w.diagnostics
	if diagnostics == nil {
		diagnostics = os.Stderr
	}

	if err != nil {
		if !w.failing || time.Since(w.lastFailure) >= time.Minute {
			_, _ = fmt.Fprintln(diagnostics, "logger.write.failed:", logsafe.Text(err.Error(), logsafe.ErrorLimit))
			w.lastFailure = time.Now()
		}

		w.failing = true
		// Already diagnosed independently; suppress Zap's unthrottled per-write stderr fallback.
		return len(p), nil
	}

	if w.failing {
		_, _ = fmt.Fprintln(diagnostics, "logger.write.recovered")
		w.failing = false
	}

	return n, nil
}

func (w *diagnosticWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	if w.writer != nil {
		return w.writer.Close()
	}

	return nil
}

// Provide wires configuration subscriptions and cleanup into the process lifecycle.
func Provide(cfg *config.Logger, provider *infraconfig.Provider) (*Logger, func(), error) {
	if cfg != nil && cfg.Service == "" && provider != nil {
		copyCfg := *cfg
		if root := provider.Get(); root != nil && root.Server != nil {
			copyCfg.Service = root.Server.Name
		}

		cfg = &copyCfg
	}

	lg, err := New(cfg)
	if err != nil {
		return nil, nil, err
	}

	if provider != nil {
		provider.SetLogger(lg.Zap())
		lg.detachLegacy = legacylog.Attach(lg.Zap())
		lg.unsubscribe = provider.Subscribe(func(next *config.Config) {
			if next != nil && next.Logger != nil {
				_ = lg.SetLevel(next.Logger.EffectiveLevel())
				lg.redactor.Update(logsafe.Secrets(next)...)
			}
		})
	}

	return lg, func() {
		if provider != nil {
			provider.Close()
		}

		lg.Stop()
	}, nil
}

// ProvideZap exposes Zap for explicit constructor injection.
func ProvideZap(l *Logger) *zap.Logger { return l.Zap() }

// limitDebug samples only repeated Debug diagnostics; all higher-level events remain unsampled.
func limitDebug(core zapcore.Core) zapcore.Core {
	return &debugCore{Core: core, sampled: zapcore.NewSamplerWithOptions(core, time.Second, 10, 100)}
}

type debugCore struct {
	zapcore.Core
	sampled zapcore.Core
}

func (c *debugCore) With(fields []zapcore.Field) zapcore.Core { return limitDebug(c.Core.With(fields)) }

func (c *debugCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if entry.Level == zap.DebugLevel {
		return c.sampled.Check(entry, checked)
	}

	return c.Core.Check(entry, checked)
}

func newCondition(l *Logger, level, candidate zapcore.Level) bool {
	return l.level.Enabled(candidate) && (candidate == level || (level == zap.ErrorLevel && candidate >= zap.ErrorLevel))
}

func newCondition2(value string) bool {
	return strings.ContainsAny(value, `/\`) || value == "." || value == ".."
}

func newCondition3(existing os.FileInfo, statErr error, fresh os.FileInfo, newErr error) bool {
	return statErr == nil && newErr == nil && os.SameFile(existing, fresh)
}

func finishNew(cfg *config.Logger, l *Logger, service, env, instance string) (*Logger, error) {
	var cores []zapcore.Core

	if cfg.EffectiveOutput() != "stdout" {
		files, err := l.fileCores(cfg, service, instance)
		if err != nil {
			return nil, err
		}

		cores = append(cores, files...)
	}

	if cfg.EffectiveOutput() != "files" {
		l.console = &diagnosticWriter{target: os.Stdout}
		cores = append(cores, zapcore.NewCore(encoder(cfg.EffectiveFormat()), zapcore.AddSync(l.console), l.level))
	}

	l.redactor = logsafe.New()
	core := logsafe.Core(limitDebug(zapcore.NewTee(cores...)), l.redactor)
	l.lg = zap.New(core, zap.AddCaller(), zap.ErrorOutput(zapcore.Lock(os.Stderr))).With(zap.String("service", service), zap.String("env", env), zap.String("instance", instance), zap.String("component", "application"))

	return l, nil
}

func prepareLogPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if err = os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return "", fmt.Errorf("create log directory: %w", err)
	}

	realDir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}

	abs = filepath.Join(realDir, filepath.Base(abs))
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}

	return abs, nil
}

func (l *Logger) fileCores(cfg *config.Logger, service, instance string) ([]zapcore.Core, error) {
	var cores []zapcore.Core

	seen := map[string]bool{}

	infos := []struct {
		path  string
		level zapcore.Level
	}{{cfg.DebugFileName, zap.DebugLevel}, {cfg.InfoFileName, zap.InfoLevel}, {cfg.WarnFileName, zap.WarnLevel}, {cfg.ErrorFileName, zap.ErrorLevel}}
	for _, info := range infos {
		path := strings.NewReplacer("{service}", service, "{instance}", instance).Replace(info.path)

		abs, err := prepareLogPath(path)
		if err != nil {
			l.Stop()
			return nil, err
		}

		if seen[abs] {
			l.Stop()
			return nil, errors.New("logger level files resolve to the same path")
		}

		seen[abs] = true

		lock, err := os.OpenFile(abs+".lock", os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- Output paths come from validated administrator configuration and are canonicalized before locking.
		if err != nil {
			l.Stop()
			return nil, fmt.Errorf("open log lock: %w", err)
		}

		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = lock.Close()

			l.Stop()

			return nil, fmt.Errorf("log file already owned by another process: %w", err)
		}

		l.locks = append(l.locks, lock)

		f, err := os.OpenFile(abs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304 -- Output paths come from validated administrator configuration and are canonicalized before locking.
		if err != nil {
			l.Stop()
			return nil, fmt.Errorf("open log output: %w", err)
		}

		for _, held := range l.writers {
			existing, statErr := os.Stat(held.writer.Filename)

			fresh, newErr := f.Stat()
			if newCondition3(existing, statErr, fresh, newErr) {
				_ = f.Close()

				l.Stop()

				return nil, errors.New("logger files share the same inode")
			}
		}

		if err = f.Close(); err != nil {
			l.Stop()
			return nil, err
		}

		writer := &diagnosticWriter{writer: &lumberjack.Logger{Filename: abs, MaxSize: cfg.MaxSize, MaxAge: cfg.MaxAge, MaxBackups: cfg.MaxBackups, Compress: true}}
		l.writers = append(l.writers, writer)
		level := info.level
		enabled := zap.LevelEnablerFunc(func(candidate zapcore.Level) bool {
			return newCondition(l, level, candidate)
		})
		cores = append(cores, zapcore.NewCore(encoder("json"), zapcore.AddSync(writer), enabled))
	}

	return cores, nil
}
