package ctxutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDetachedTaskGroupWaitsUntilIdle(t *testing.T) {
	group := NewDetachedTaskGroup()
	started := make(chan struct{})
	release := make(chan struct{})

	if !group.Go(t.Context(), time.Second, func(context.Context) {
		close(started)
		<-release
	}) {
		t.Fatal("expected task to be accepted")
	}
	<-started

	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	waited := make(chan error, 1)
	go func() {
		waited <- group.Wait(waitCtx)
	}()

	select {
	case err := <-waited:
		t.Fatalf("wait returned before task completed: %v", err)
	default:
	}

	close(release)
	if err := <-waited; err != nil {
		t.Fatalf("wait failed: %v", err)
	}
}

func TestDetachedTaskGroupStopCancelsAndRejectsTasks(t *testing.T) {
	group := NewDetachedTaskGroup()
	started := make(chan struct{})
	finished := make(chan struct{})

	if !group.Go(t.Context(), time.Second, func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(finished)
	}) {
		t.Fatal("expected task to be accepted")
	}
	<-started

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := group.Stop(stopCtx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	select {
	case <-finished:
	default:
		t.Fatal("stop returned before canceled task exited")
	}

	if group.Go(t.Context(), time.Second, func(context.Context) {}) {
		t.Fatal("expected task to be rejected after stop")
	}
}

func TestDetachedTaskGroupWaitHonorsContext(t *testing.T) {
	group := NewDetachedTaskGroup()
	release := make(chan struct{})
	defer close(release)

	if !group.Go(t.Context(), time.Second, func(context.Context) {
		<-release
	}) {
		t.Fatal("expected task to be accepted")
	}

	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := group.Wait(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context.Canceled", err)
	}
}
