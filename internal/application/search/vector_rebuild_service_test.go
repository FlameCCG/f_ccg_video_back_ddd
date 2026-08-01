package search

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestVideoVectorRebuildLogsItemFailureAndCompletion(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	tasks := ctxutil.NewDetachedTaskGroup()
	storeErr := errors.New("store vector failed")
	vectorIndex := &fakeVectorIndex{available: true, storeErr: storeErr}
	vectors := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		vectorIndex,
		&fakeVectorSource{available: true},
		logger,
	)
	rebuild := NewVideoVectorRebuildService(
		vectors,
		&fakeVectorRebuildSource{sources: []VideoVectorSource{{
			ID:     7,
			Status: enum.VideoStatusPublished,
		}}},
		&fakeVectorLifecycle{available: true},
		tasks,
		logger,
	)

	started, err := rebuild.StartRebuild(RebuildOptions{Workers: 1})
	if err != nil || !started {
		t.Fatalf("StartRebuild() = (%v, %v), want (true, nil)", started, err)
	}
	waitForRebuildTasks(t, tasks)

	warnings := logs.FilterMessage("rebuild video vector failed").All()
	if len(warnings) != 1 {
		t.Fatalf("warning count = %d, want 1", len(warnings))
	}
	warningFields := warnings[0].ContextMap()
	if fmt.Sprint(warningFields["videoId"]) != "7" ||
		fmt.Sprint(warningFields["error"]) != storeErr.Error() {
		t.Fatalf("warning fields = %#v", warningFields)
	}

	completed := logs.FilterMessage("video vector rebuild finished").All()
	if len(completed) != 1 {
		t.Fatalf("completion log count = %d, want 1", len(completed))
	}
	completionFields := completed[0].ContextMap()
	if completionFields["succeeded"] != int64(0) || completionFields["failed"] != int64(1) {
		t.Fatalf("completion fields = %#v", completionFields)
	}
	if _, exists := completionFields["error"]; exists {
		t.Fatalf("successful run must not carry a run error: %#v", completionFields)
	}

	status := rebuild.RebuildStatus()
	if status.Running || status.Succeeded != 0 || status.Failed != 1 || status.LastError != "" {
		t.Fatalf("status = %#v", status)
	}
}

func TestVideoVectorRebuildCompletionLogsRunError(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	logger := zap.New(core)
	tasks := ctxutil.NewDetachedTaskGroup()
	prepareErr := errors.New("prepare index failed")
	vectorIndex := &fakeVectorIndex{available: true}
	vectors := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		vectorIndex,
		&fakeVectorSource{available: true},
		logger,
	)
	rebuild := NewVideoVectorRebuildService(
		vectors,
		&fakeVectorRebuildSource{},
		&fakeVectorLifecycle{available: true, prepareErr: prepareErr},
		tasks,
		logger,
	)

	started, err := rebuild.StartRebuild(RebuildOptions{})
	if err != nil || !started {
		t.Fatalf("StartRebuild() = (%v, %v), want (true, nil)", started, err)
	}
	waitForRebuildTasks(t, tasks)

	completed := logs.FilterMessage("video vector rebuild finished").All()
	if len(completed) != 1 {
		t.Fatalf("completion log count = %d, want 1", len(completed))
	}
	fields := completed[0].ContextMap()
	if fields["succeeded"] != int64(0) || fields["failed"] != int64(0) ||
		fmt.Sprint(fields["error"]) != prepareErr.Error() {
		t.Fatalf("completion fields = %#v", fields)
	}

	status := rebuild.RebuildStatus()
	if status.LastError != prepareErr.Error() {
		t.Fatalf("LastError = %q, want %q", status.LastError, prepareErr)
	}
}

func TestNewVideoVectorRebuildServiceUsesNopLoggerWhenNil(t *testing.T) {
	t.Parallel()

	service := NewVideoVectorRebuildService(nil, nil, nil, nil, nil)
	if service.logger == nil {
		t.Fatal("logger = nil, want Nop logger")
	}
}

func waitForRebuildTasks(t *testing.T, tasks *ctxutil.DetachedTaskGroup) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tasks.Wait(ctx); err != nil {
		t.Fatalf("wait for rebuild: %v", err)
	}
}

type fakeVectorRebuildSource struct {
	sources  []VideoVectorSource
	countErr error
	loadErr  error
}

func (*fakeVectorRebuildSource) Available() bool { return true }

func (f *fakeVectorRebuildSource) CountPublished(context.Context) (int64, error) {
	return int64(len(f.sources)), f.countErr
}

func (f *fakeVectorRebuildSource) PublishedAfter(
	_ context.Context,
	cursor uint,
	_ int,
) ([]VideoVectorSource, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}

	result := make([]VideoVectorSource, 0, len(f.sources))
	for _, source := range f.sources {
		if source.ID > cursor {
			result = append(result, source)
		}
	}

	return result, nil
}

type fakeVectorLifecycle struct {
	available  bool
	prepareErr error
	refreshErr error
}

func (f *fakeVectorLifecycle) Available() bool { return f.available }

func (f *fakeVectorLifecycle) Prepare(context.Context, VectorConfig, bool) error {
	return f.prepareErr
}

func (f *fakeVectorLifecycle) Refresh(context.Context, VectorConfig) error {
	return f.refreshErr
}
