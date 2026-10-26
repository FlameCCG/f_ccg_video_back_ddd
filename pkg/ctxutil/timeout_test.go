package ctxutil

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithDetachedTimeoutIgnoresParentCancel(t *testing.T) {
	t.Parallel()

	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()

	ctx, cancel := WithDetachedTimeout(parent, 50*time.Millisecond)
	defer cancel()

	{
		err := ctx.Err()
		require.NoError(t, err, "detached ctx should not inherit parent cancel, got %v", err)
	}

	<-ctx.Done()

	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded, "expected deadline exceeded, got %v", ctx.Err())
}

func TestWithTimeoutFallsBackToBackground(t *testing.T) {
	t.Parallel()

	ctx, cancel := WithTimeout(context.TODO(), 20*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded, "expected deadline exceeded, got %v", ctx.Err())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected ctx deadline to fire")
	}
}
