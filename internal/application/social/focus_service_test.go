package social

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/social"

	"go.uber.org/zap"
)

type focusRepoStub struct {
	deleted int64
}

func (r *focusRepoStub) Create(context.Context, *domain.Focus) error { return nil }

func (r *focusRepoStub) Delete(context.Context, uint, uint) (int64, error) {
	return r.deleted, nil
}

type userExistsStub struct{}

func (userExistsStub) Exists(context.Context, uint) (bool, error) { return true, nil }

type fansMetricSpy struct {
	deltas []int64
}

func (s *fansMetricSpy) IncrFans(_ context.Context, _ uint, delta int64) error {
	s.deltas = append(s.deltas, delta)

	return nil
}

type userSearchSpy struct {
	calls int
}

func (s *userSearchSpy) SyncUserDocuments(context.Context, ...uint) error {
	s.calls++

	return nil
}

func TestFocusServiceLeavesSearchIndexingToCDC(t *testing.T) {
	t.Parallel()

	search := &userSearchSpy{}
	metrics := &fansMetricSpy{}
	service := NewFocusService(&focusRepoStub{deleted: 1}, userExistsStub{}, metrics, search, zap.NewNop())

	{
		err := service.Focus(t.Context(), 1, 2)
		require.NoError(t, err, "Focus: %v", err)
	}

	{
		err := service.Unfocus(t.Context(), 1, 2)
		require.NoError(t, err, "Unfocus: %v", err)
	}

	require.Equal(t, 0, search.calls, "synchronous search calls = %d, want 0", search.calls)

	require.Len(t, metrics.deltas, 2, "metric deltas = %v, want [1 -1]", metrics.deltas)
	require.EqualValues(t, 1, metrics.deltas[0], "metric deltas = %v, want [1 -1]", metrics.deltas)
	require.EqualValues(t, -1, metrics.deltas[1], "metric deltas = %v, want [1 -1]", metrics.deltas)
}
