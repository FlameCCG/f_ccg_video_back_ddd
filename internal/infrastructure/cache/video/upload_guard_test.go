package video

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUploadHashGuardSkipsActiveCleanupAndReleases(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err, "active hash should be skipped: acquired=%v called=%v err=%v", acquired, called, err)
	require.False(t, acquired, "active hash should be skipped: acquired=%v called=%v err=%v", acquired, called, err)
	require.False(t, called, "active hash should be skipped: acquired=%v called=%v err=%v", acquired, called, err)

	close(release)

	{
		err := <-done
		require.NoError(t, err)
	}

	acquired, err = guard.TryRun(t.Context(), "hash", func(context.Context) error { called = true; return nil })
	require.NoError(t, err, "released hash should run: acquired=%v called=%v err=%v", acquired, called, err)
	require.True(t, acquired, "released hash should run: acquired=%v called=%v err=%v", acquired, called, err)
	require.True(t, called, "released hash should run: acquired=%v called=%v err=%v", acquired, called, err)
}
