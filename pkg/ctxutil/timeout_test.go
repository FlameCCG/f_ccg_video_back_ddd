package ctxutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestIsTimeout(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "wrapped deadline", err: fmt.Errorf("lookup: %w", context.DeadlineExceeded), want: true},
		{name: "transport timeout", err: fmt.Errorf("upstream: %w", &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}), want: true},
		{name: "cancelled", err: context.Canceled},
		{name: "other error", err: errors.New("connection refused")},
		{name: "nil"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTimeout(tc.err); got != tc.want {
				t.Fatalf("IsTimeout(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestWithDetachedTimeoutIgnoresParentCancel(t *testing.T) {
	parent, parentCancel := context.WithCancel(context.Background())
	parentCancel()

	ctx, cancel := WithDetachedTimeout(parent, 50*time.Millisecond)
	defer cancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("detached ctx should not inherit parent cancel, got %v", err)
	}

	<-ctx.Done()

	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", ctx.Err())
	}
}

func TestWithTimeoutFallsBackToBackground(t *testing.T) {
	ctx, cancel := WithTimeout(context.TODO(), 20*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got %v", ctx.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected ctx deadline to fire")
	}
}
