package social

import (
	"context"
	"testing"

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
	search := &userSearchSpy{}
	metrics := &fansMetricSpy{}
	service := NewFocusService(&focusRepoStub{deleted: 1}, userExistsStub{}, metrics, search, zap.NewNop())

	if err := service.Focus(t.Context(), 1, 2); err != nil {
		t.Fatalf("Focus: %v", err)
	}

	if err := service.Unfocus(t.Context(), 1, 2); err != nil {
		t.Fatalf("Unfocus: %v", err)
	}

	if search.calls != 0 {
		t.Fatalf("synchronous search calls = %d, want 0", search.calls)
	}

	if len(metrics.deltas) != 2 || metrics.deltas[0] != 1 || metrics.deltas[1] != -1 {
		t.Fatalf("metric deltas = %v, want [1 -1]", metrics.deltas)
	}
}
