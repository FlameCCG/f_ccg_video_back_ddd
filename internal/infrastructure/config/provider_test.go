package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
)

const validFileConfig = `transcode:
  localConsumerDisabled: true
jwt:
  accessSecret: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  refreshSecret: bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
site:
  login:
    slideCaptchaPadding: 0
`

func TestLoadOnceReadsValidatedConfigAndDefaults(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(validFileConfig), 0600); err != nil {
		t.Fatal(err)
	}
	provider, err := LoadOnce(path)
	if err != nil {
		t.Fatal(err)
	}
	if provider.Path() != path {
		t.Fatalf("Path() = %q, want %q", provider.Path(), path)
	}
	cfg := provider.Get()
	if cfg == nil || cfg.Transcode == nil || !cfg.Transcode.LocalConsumerDisabled {
		t.Fatalf("transcode config was not loaded: %+v", cfg)
	}
	if cfg.Site.Login.SlideCaptchaTTL != 300 || cfg.Site.Register.TextClickCaptchaPadding != 20 {
		t.Fatal("captcha defaults were not loaded")
	}
	if cfg.Site.Login.SlideCaptchaPadding != 0 {
		t.Fatal("an explicit zero padding must override the default")
	}
}

func TestConfigLoadersReturnInitializationErrors(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		content string
		want    string
	}{
		{name: "missing", want: "read config file"},
		{name: "malformed", content: "jwt: [", want: "read config file"},
		{name: "invalid", content: "transcode:\n  localConsumerDisabled: true\n", want: "config validation error"},
		{name: "mapping", content: strings.Replace(validFileConfig, "slideCaptchaPadding: 0", "slideCaptchaPadding: invalid", 1), want: "config unmarshal error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if test.content != "" {
				if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			loaders := []func(string) (*Provider, error){Load, LoadOnce, LoadTranscodeWorker}
			if test.name == "invalid" {
				loaders = []func(string) (*Provider, error){Load}
			}
			for _, loadConfig := range loaders {
				provider, err := loadConfig(path)
				if provider != nil || err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("load = (%v, %v), want nil provider and %q error", provider, err, test.want)
				}
			}
		})
	}
}

func TestConfigLoadersValidateByRole(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name                            string
		content                         string
		httpOK, workerOK, maintenanceOK bool
	}{
		{name: "database maintenance", content: "mysql:\n  host: localhost\n", maintenanceOK: true},
		{name: "consumer without JWT", content: "transcode:\n  localConsumerDisabled: false\n", workerOK: true, maintenanceOK: true},
		{name: "HTTP", content: validFileConfig, httpOK: true, workerOK: true, maintenanceOK: true},
		{name: "weak HTTP secret", content: strings.Replace(validFileConfig, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "weak", 1), workerOK: true, maintenanceOK: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			for _, role := range []struct {
				name string
				load func(string) (*Provider, error)
				ok   bool
			}{
				{"HTTP", Load, test.httpOK},
				{"consumer", LoadTranscodeWorker, test.workerOK},
				{"maintenance", LoadOnce, test.maintenanceOK},
			} {
				_, err := role.load(path)
				if (err == nil) != role.ok {
					t.Fatalf("%s load error = %v, want success %v", role.name, err, role.ok)
				}
			}
		})
	}
}

// TestProviderAtomicLoadStore 对应原 global.TestConfigAtomicLoadStore。
func TestProviderAtomicLoadStore(t *testing.T) {
	t.Parallel()

	provider := NewProvider(nil)

	first := &config.Config{Jwt: &config.Jwt{AccessSecret: "a"}}
	provider.Set(first)

	if got := provider.Get(); got != first {
		t.Fatalf("Get() = %p, want %p", got, first)
	}

	second := &config.Config{Jwt: &config.Jwt{AccessSecret: "b"}}
	provider.Set(second)

	if got := provider.Get(); got != second {
		t.Fatalf("Get() after replace = %p, want %p", got, second)
	}
}

// TestProviderConcurrentHotReload 对应原 global.TestConfigConcurrentHotReload。
func TestProviderConcurrentHotReload(t *testing.T) {
	t.Parallel()

	provider := NewProvider(&config.Config{})

	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 200 {
				provider.Set(&config.Config{})

				_ = provider.Get()
			}
		}()
	}

	wg.Wait()
}

// TestProviderNilReceiver 保证 nil Provider 的读写不 panic。
func TestProviderNilReceiver(t *testing.T) {
	t.Parallel()

	var provider *Provider

	if got := provider.Get(); got != nil {
		t.Fatalf("nil provider Get() = %v, want nil", got)
	}

	provider.Set(&config.Config{})

	if got := provider.Path(); got != "" {
		t.Fatalf("nil provider Path() = %q, want empty", got)
	}
}
