package config

import (
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	testDebug  = "debug"
	testStdout = "stdout"
)

func TestPersistenceFailureKeepsPreviousSnapshotAndNotifications(t *testing.T) {
	t.Parallel()

	original := &config.Config{Logger: &config.Logger{Output: testStdout, Level: "info"}}
	p := NewProvider(original)

	var notifications atomic.Int64

	unsubscribe := p.Subscribe(func(*config.Config) { notifications.Add(1) })
	defer unsubscribe()

	changed := &config.Logger{Output: testStdout, Level: testDebug}

	cause := errors.New("disk unavailable")
	{
		err := p.PublishSection("logger", changed, func() error { return cause })
		require.ErrorIs(t, err, cause, "persistence cause=%v", err)
	}

	require.Same(t, p.Get(), original, "failed persistence published a new snapshot")
	require.EqualValues(t, 1, notifications.Load(), "failed persistence published a new snapshot")

	invalid := &config.Logger{Output: "invalid"}
	{
		err := p.PublishSection("logger", invalid, func() error { t.Fatal("invalid config persisted"); return nil })
		require.Error(t, err, "invalid logger config accepted")
	}

	{
		err := p.PublishSection("logger", changed, func() error { return nil })
		require.NoError(t, err)
	}

	require.EqualValues(t, 2, notifications.Load(), "successful update not notified")
	require.Equal(t, testDebug, p.Get().Logger.Level, "successful update not notified")

	require.False(t, p.LoggerRestartRequired(changed), "level-only update requires restart")

	outputs := *changed

	outputs.ConsoleFormat = "console"
	require.True(t, p.LoggerRestartRequired(&outputs), "format update does not require restart")
}

func TestConcurrentSectionUpdatesMergeAndUnsubscribe(t *testing.T) {
	t.Parallel()

	p := NewProvider(&config.Config{Logger: &config.Logger{Output: testStdout}})

	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			for range 25 {
				name := "logger"

				var section any = &config.Logger{Output: testStdout, Level: testDebug}

				if index == 1 {
					name = "server"
					section = &config.Server{Name: "updated"}
				}

				{
					err := p.PublishSection(name, section, func() error { return nil })
					assert.NoError(t, err)
				}
			}
		}(i)
	}

	wg.Wait()

	require.Equal(t, testDebug, p.Get().Logger.Level, "concurrent section update lost")
	require.Equal(t, "updated", p.Get().Server.Name, "concurrent section update lost")

	var called atomic.Int64

	off := p.Subscribe(func(*config.Config) { called.Add(1) })
	off()
	p.Set(p.Get())

	require.EqualValues(t, 1, called.Load(), "unsubscribed callback ran")

	p.Close()
	p.Close()
}

func TestFileReloadUsesNotificationsAndKeepsInvalidSnapshotOut(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")

	content := func(level string) []byte {
		return []byte("jwt:\n  accessSecret: " + strings.Repeat("a", 32) + "\n  refreshSecret: " + strings.Repeat("b", 32) + "\ntranscode:\n  mp4Enable: true\nlogger:\n  output: stdout\n  level: " + level + "\n")
	}
	{
		err := os.WriteFile(path, content("info"), 0o600)
		require.NoError(t, err)
	}

	p, err := Load(path)
	require.NoError(t, err)

	defer p.Close()

	core, logs := observer.New(zap.DebugLevel)
	p.SetLogger(zap.New(core))
	original := p.Get()
	changed := make(chan string, 4)

	off := p.Subscribe(func(c *config.Config) { changed <- c.Logger.EffectiveLevel() })
	defer off()

	<-changed

	{
		err := os.WriteFile(path, content("invalid"), 0o600)
		require.NoError(t, err)
	}

	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()

	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()

	for logs.FilterMessage("config.reload.failed").Len() == 0 {
		select {
		case <-deadline.C:
			t.Fatal("invalid reload was not diagnosed")
		case <-tick.C:
		}
	}

	require.Same(t, p.Get(), original, "invalid file replaced the active snapshot")

	select {
	case level := <-changed:
		t.Fatalf("invalid config notified: %s", level)
	default:
	}

	{
		err := os.WriteFile(path, content(testDebug), 0o600)
		require.NoError(t, err)
	}

	select {
	case level := <-changed:
		require.Equal(t, testDebug, level, "notification=%s", level)
	case <-deadline.C:
		t.Fatal("valid reload was not notified")
	}
}
