package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/infrastructure/mq"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

type workerConsumerStub struct {
	start func() error
	stop  func() error
}

func (s workerConsumerStub) Start() error { return s.start() }
func (s workerConsumerStub) Stop() error  { return s.stop() }

func TestTranscodeWorkerWaitsForCancellationAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	stopped := false
	worker := &TranscodeWorker{
		consumer: workerConsumerStub{
			start: func() error { close(started); return nil },
			stop:  func() error { stopped = true; return nil },
		},
		producers: mq.NewProducerPool(zap.NewNop()),
		detached:  ctxutil.NewDetachedTaskGroup(),
		logger:    zap.NewNop(),
	}
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("consumer did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("worker exited before cancellation: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop")
	}
	if !stopped {
		t.Fatal("consumer was not stopped before Run returned")
	}
	if worker.detached.Go(context.Background(), time.Second, func(context.Context) {}) {
		t.Fatal("storage task group still accepts work after shutdown")
	}
}

func TestTranscodeWorkerCleansUpOnStartFailureOrCanceledContext(t *testing.T) {
	startErr := errors.New("subscription failed")
	stopErr := errors.New("close failed")
	for _, tt := range []struct {
		name      string
		canceled  bool
		startErr  error
		stopErr   error
		wantStart int
	}{
		{name: "start failure", startErr: startErr, wantStart: 1},
		{name: "start and stop failures preserved", startErr: startErr, stopErr: stopErr, wantStart: 1},
		{name: "already canceled", canceled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tt.canceled {
				cancel()
			}
			var starts, stops int
			worker := &TranscodeWorker{
				consumer: workerConsumerStub{
					start: func() error { starts++; return tt.startErr },
					stop:  func() error { stops++; return tt.stopErr },
				},
				logger: zap.NewNop(),
			}
			err := worker.Run(ctx)
			if starts != tt.wantStart || stops != 1 {
				t.Fatalf("starts = %d, stops = %d", starts, stops)
			}
			if tt.startErr != nil && !errors.Is(err, tt.startErr) {
				t.Fatalf("start error lost: %v", err)
			}
			if tt.stopErr != nil && !errors.Is(err, tt.stopErr) {
				t.Fatalf("stop error lost: %v", err)
			}
			if tt.startErr == nil && tt.stopErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
