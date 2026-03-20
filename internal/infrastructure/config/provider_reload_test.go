package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const hotReloadConfig = `transcode:
  localConsumerDisabled: true
jwt:
  accessSecret: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  refreshSecret: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
site:
  login:
    slideCaptchaPadding: 0
server:
  name: initial
`

func newHotReloadProvider(t *testing.T) (*infraconfig.Provider, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeHotReloadConfig(t, path, hotReloadConfig)
	provider, err := infraconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return provider, path
}

func writeHotReloadConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func waitForHotReload(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("configuration watcher did not reload the file")
}

func TestHotReloadSurvivesAtomicConfigWrites(t *testing.T) {
	provider, path := newHotReloadProvider(t)
	writer := contentops.NewConfigFileWriter(path)
	for _, name := range []string{"first", "second"} {
		if err := writer.Write("server", &config.Server{Name: name}); err != nil {
			t.Fatal(err)
		}
		waitForHotReload(t, func() bool {
			return provider.Get().Server.Name == name
		})
	}

	// 管理端连续原子替换之后，手工编辑仍应触发热更新。
	writeHotReloadConfig(t, path, strings.Replace(hotReloadConfig, "name: initial", "name: manual", 1))
	waitForHotReload(t, func() bool {
		return provider.Get().Server.Name == "manual"
	})
	cfg := provider.Get()
	if cfg.Site.Login.SlideCaptchaTTL != 300 || cfg.Site.Login.SlideCaptchaPadding != 0 {
		t.Fatal("hot reload lost defaults or an explicit zero value")
	}
}

func TestHotReloadKeepsSnapshotAndRecoversAfterInvalidConfig(t *testing.T) {
	for name, invalid := range map[string]string{
		"malformed YAML": "jwt: [",
		"invalid JWT":    strings.Replace(hotReloadConfig, strings.Repeat("a", 32), "weak", 1),
	} {
		t.Run(name, func(t *testing.T) {
			provider, path := newHotReloadProvider(t)
			core, logs := observer.New(zap.ErrorLevel)
			provider.SetLogger(zap.New(core))
			previous := provider.Get()
			writeHotReloadConfig(t, path, invalid)
			waitForHotReload(t, func() bool { return logs.Len() > 0 })
			if provider.Get() != previous {
				t.Fatal("invalid configuration replaced the last valid snapshot")
			}

			writeHotReloadConfig(t, path, strings.Replace(hotReloadConfig, "name: initial", "name: recovered", 1))
			waitForHotReload(t, func() bool {
				return provider.Get().Server.Name == "recovered"
			})
		})
	}
}

func TestHotReloadSurvivesConfigRemovalAndRecreation(t *testing.T) {
	provider, path := newHotReloadProvider(t)
	core, logs := observer.New(zap.ErrorLevel)
	provider.SetLogger(zap.New(core))
	previous := provider.Get()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitForHotReload(t, func() bool { return logs.Len() > 0 })
	if provider.Get() != previous {
		t.Fatal("removing the file changed the published snapshot")
	}
	writeHotReloadConfig(t, path, strings.Replace(hotReloadConfig, "name: initial", "name: recreated", 1))
	waitForHotReload(t, func() bool {
		return provider.Get().Server.Name == "recreated"
	})
}
