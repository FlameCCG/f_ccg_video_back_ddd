package ctxutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

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
