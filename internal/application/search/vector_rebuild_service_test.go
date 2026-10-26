package search

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	started, err := rebuild.StartRebuild(t.Context(), RebuildOptions{Workers: 1})
	require.NoError(t, err, "StartRebuild() = (%v, %v), want (true, nil)", started, err)
	require.True(t, started, "StartRebuild() = (%v, %v), want (true, nil)", started, err)

	waitForRebuildTasks(t, tasks)

	warnings := logs.FilterMessage("rebuild video vector failed").All()
	require.Len(t, warnings, 1, "warning count = %d, want 1", len(warnings))

	warningFields := warnings[0].ContextMap()
	require.Equal(t, "7", fmt.Sprint(warningFields["video_id"]), "warning fields = %#v", warningFields)
	require.Equal(t, fmt.Sprint(warningFields["error"]), storeErr.Error(), "warning fields = %#v", warningFields)

	completed := logs.FilterMessage("video vector rebuild finished").All()
	require.Len(t, completed, 1, "completion log count = %d, want 1", len(completed))

	completionFields := completed[0].ContextMap()
	require.EqualValues(t, int64(0), completionFields["succeeded"], "completion fields = %#v", completionFields)
	require.EqualValues(t, int64(1), completionFields["failed"], "completion fields = %#v", completionFields)

	{
		_, exists := completionFields["error"]
		require.False(t, exists, "successful run must not carry a run error: %#v", completionFields)
	}

	status := rebuild.RebuildStatus()
	require.False(t, status.Running, "status = %#v", status)
	require.EqualValues(t, 0, status.Succeeded, "status = %#v", status)
	require.EqualValues(t, 1, status.Failed, "status = %#v", status)
	require.Empty(t, status.LastError, "status = %#v", status)
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

	started, err := rebuild.StartRebuild(t.Context(), RebuildOptions{})
	require.NoError(t, err, "StartRebuild() = (%v, %v), want (true, nil)", started, err)
	require.True(t, started, "StartRebuild() = (%v, %v), want (true, nil)", started, err)

	waitForRebuildTasks(t, tasks)

	completed := logs.FilterMessage("video vector rebuild finished").All()
	require.Len(t, completed, 1, "completion log count = %d, want 1", len(completed))

	fields := completed[0].ContextMap()
	require.EqualValues(t, int64(0), fields["succeeded"], "completion fields = %#v", fields)
	require.EqualValues(t, int64(0), fields["failed"], "completion fields = %#v", fields)
	require.Equal(t, fmt.Sprint(fields["error"]), prepareErr.Error(), "completion fields = %#v", fields)

	status := rebuild.RebuildStatus()
	require.Equal(t, status.LastError, prepareErr.Error(), "LastError = %q, want %q", status.LastError, prepareErr)
}

func TestNewVideoVectorRebuildServiceUsesNopLoggerWhenNil(t *testing.T) {
	t.Parallel()

	service := NewVideoVectorRebuildService(nil, nil, nil, nil, nil)
	require.NotNil(t, service.logger, "logger = nil, want Nop logger")
}

func waitForRebuildTasks(t *testing.T, tasks *ctxutil.DetachedTaskGroup) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	{
		err := tasks.Wait(ctx)
		require.NoError(t, err, "wait for rebuild: %v", err)
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
