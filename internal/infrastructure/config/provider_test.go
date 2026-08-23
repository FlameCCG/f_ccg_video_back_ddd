package config

import (
	"sync"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
)

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
