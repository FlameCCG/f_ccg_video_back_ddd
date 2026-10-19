package video

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestUploadHashGuardSkipsActiveCleanupAndReleases(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	guard := NewUploadHashGuard(client)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- guard.Run(context.Background(), []string{"hash"}, func(context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	called := false
	acquired, err := guard.TryRun(t.Context(), "hash", func(context.Context) error { called = true; return nil })
	if err != nil || acquired || called {
		t.Fatalf("active hash should be skipped: acquired=%v called=%v err=%v", acquired, called, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	acquired, err = guard.TryRun(t.Context(), "hash", func(context.Context) error { called = true; return nil })
	if err != nil || !acquired || !called {
		t.Fatalf("released hash should run: acquired=%v called=%v err=%v", acquired, called, err)
	}
}
