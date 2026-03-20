package contentops_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
)

type configWriterFunc func(string, any) error

func (f configWriterFunc) Write(name string, value any) error { return f(name, value) }

func validJWT() *config.Jwt {
	return &config.Jwt{AccessSecret: strings.Repeat("a", 32), RefreshSecret: strings.Repeat("r", 32)}
}

func TestJWTValidationBeforePersistenceAndPublication(t *testing.T) {
	for name, candidate := range map[string]*config.Jwt{
		"null":          nil,
		"short access":  {AccessSecret: "short", RefreshSecret: strings.Repeat("r", 32)},
		"short refresh": {AccessSecret: strings.Repeat("a", 32), RefreshSecret: "short"},
	} {
		t.Run(name, func(t *testing.T) {
			initial := &config.Config{Jwt: validJWT()}
			provider := infraconfig.NewProvider(initial)
			writes := 0
			service := app.NewSiteConfigService(provider, configWriterFunc(func(string, any) error { writes++; return nil }))
			if err := service.UpdateJWT(candidate); err == nil {
				t.Fatal("invalid JWT accepted")
			}
			if provider.Get() != initial || writes != 0 {
				t.Fatal("invalid JWT changed memory or disk")
			}
		})
	}
}

func TestJWTMaskedSecretsRetainValidValues(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{Jwt: validJWT()})
	service := app.NewSiteConfigService(provider, configWriterFunc(func(string, any) error { return nil }))
	if err := service.UpdateJWT(&config.Jwt{AccessSecret: "******", RefreshSecret: ""}); err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateJWTConfig(provider.Get().Jwt); err != nil {
		t.Fatal(err)
	}
}

func TestConfigWriteFailureKeepsPublishedSnapshot(t *testing.T) {
	initial := &config.Config{Jwt: validJWT()}
	provider := infraconfig.NewProvider(initial)
	want := errors.New("disk full")
	service := app.NewSiteConfigService(provider, configWriterFunc(func(string, any) error {
		if provider.Get() != initial {
			t.Error("snapshot published before persistence")
		}
		return want
	}))
	if err := service.UpdateJWT(&config.Jwt{AccessSecret: strings.Repeat("b", 32), RefreshSecret: strings.Repeat("c", 32)}); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if provider.Get() != initial {
		t.Fatal("failed update changed memory")
	}
}

func TestConcurrentServicesPreserveBothConfigSections(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{Jwt: validJWT()})
	entered, release := make(chan struct{}), make(chan struct{})
	writer := configWriterFunc(func(name string, _ any) error {
		if name == app.ConfigNameJWT {
			close(entered)
			<-release
		}
		return nil
	})
	first, second := app.NewSiteConfigService(provider, writer), app.NewSiteConfigService(provider, writer)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := first.UpdateJWT(&config.Jwt{AccessSecret: strings.Repeat("b", 32), RefreshSecret: strings.Repeat("c", 32)}); err != nil {
			t.Error(err)
		}
	}()
	<-entered
	go func() {
		defer wg.Done()
		if err := second.UpdateServerCORS([]string{"https://example.com"}); err != nil {
			t.Error(err)
		}
	}()
	close(release)
	wg.Wait()
	cfg := provider.Get()
	if cfg.Jwt.AccessSecret != strings.Repeat("b", 32) || cfg.Server == nil || len(cfg.Server.CorsOrigins) != 1 {
		t.Fatal("concurrent updates lost a section")
	}
}
