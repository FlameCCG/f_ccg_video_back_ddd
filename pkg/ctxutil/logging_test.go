package ctxutil

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDetachedAssociationAndPanicBoundary(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	group := NewLoggedDetachedTaskGroup(zap.New(core))
	parent, cancel := context.WithCancel(logmeta.With(context.Background(), logmeta.Values{RequestID: "request", UserID: "7"}))
	cancel()

	if !group.Go(parent, time.Second, func(ctx context.Context) {
		if ctx.Err() != nil {
			t.Error("detached task inherited cancellation")
		}

		v := logmeta.From(ctx)
		if v.RequestID != "request" || v.UserID != "7" || v.TaskID == "" || v.AttemptID == "" {
			t.Errorf("lost association: %+v", v)
		}

		panic("independent task failure")
	}) {
		t.Fatal("task rejected")
	}

	if err := group.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}

	failures := logs.FilterMessage("task.failed").All()
	if len(failures) != 1 || failures[0].ContextMap()["stack"] == nil {
		t.Fatalf("task panic not captured: %#v", logs.All())
	}

	if err := group.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
}
