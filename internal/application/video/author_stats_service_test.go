package video

import (
	"context"
	"errors"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

type authorStatsQueryStub struct {
	rows  []AuthorVideoStatRow
	err   error
	calls int
}

func (s *authorStatsQueryStub) ListByAuthor(
	context.Context,
	string,
	bool,
	int,
	int,
	int,
	int,
) ([]UserVideoRow, int64, error) {
	return nil, 0, nil
}

func (s *authorStatsQueryStub) RecentActionVideos(
	context.Context,
	uint,
	string,
	int,
) ([]UserVideoRow, error) {
	return nil, nil
}

func (s *authorStatsQueryStub) AggregateAuthorStats(
	context.Context,
	string,
) ([]AuthorVideoStatRow, error) {
	s.calls++

	return s.rows, s.err
}

type authorStatsCacheStub struct {
	value  UserVideoStats
	found  bool
	stored UserVideoStats
}

func (s *authorStatsCacheStub) Load(string) (UserVideoStats, bool) {
	return s.value, s.found
}

func (s *authorStatsCacheStub) Store(_ string, stats UserVideoStats) {
	s.stored = stats
}

type statsCacheStub struct {
	values map[uint]*domain.Stats
}

func (s *statsCacheStub) Available() bool { return true }

func (s *statsCacheStub) Load(context.Context, uint) (*domain.Stats, bool) {
	return nil, false
}

func (s *statsCacheStub) LoadBatch(context.Context, []uint) map[uint]*domain.Stats {
	return s.values
}

func (s *statsCacheStub) RawPayload(context.Context, uint) (string, bool, error) {
	return "", false, nil
}

func (s *statsCacheStub) Seed(context.Context, uint, *domain.Stats) {}

func (s *statsCacheStub) ApplyDelta(
	context.Context,
	uint,
	domain.StatsDelta,
	string,
) (*domain.Stats, error) {
	return nil, nil
}

func (s *statsCacheStub) ApplyDeltaIfExists(
	context.Context,
	uint,
	domain.StatsDelta,
) (*domain.Stats, bool, error) {
	return nil, false, nil
}

func (s *statsCacheStub) Clear(context.Context, ...uint) {}

func (s *statsCacheStub) IsTransientError(error) bool { return false }

func TestAuthorStatsServiceUsesRedisValuesAndDatabaseFallback(t *testing.T) {
	t.Parallel()

	query := &authorStatsQueryStub{rows: []AuthorVideoStatRow{
		{ID: 1, Views: 10, Likes: 2},
		{ID: 2, Views: 20, Likes: 3},
	}}
	cache := &authorStatsCacheStub{}
	stats := &StatsService{cache: &statsCacheStub{
		values: map[uint]*domain.Stats{
			1: {Views: 100, Likes: 20},
		},
	}, logger: zap.NewNop()}
	service := NewAuthorStatsService(query, stats, cache)

	got := service.AggregateByAuthor(context.Background(), "author")
	want := UserVideoStats{TotalViews: 120, TotalLikes: 23}

	if got != want {
		t.Fatalf("AggregateByAuthor() = %#v, want %#v", got, want)
	}

	if cache.stored != want {
		t.Fatalf("stored stats = %#v, want %#v", cache.stored, want)
	}
}

func TestAuthorStatsServiceReturnsCachedValueWithoutQuery(t *testing.T) {
	t.Parallel()

	query := &authorStatsQueryStub{err: errors.New("must not be called")}
	want := UserVideoStats{TotalViews: 9, TotalLikes: 4}
	service := NewAuthorStatsService(
		query,
		&StatsService{logger: zap.NewNop()},
		&authorStatsCacheStub{value: want, found: true},
	)

	got := service.AggregateByAuthor(context.Background(), "author")
	if got != want {
		t.Fatalf("AggregateByAuthor() = %#v, want %#v", got, want)
	}

	if query.calls != 0 {
		t.Fatalf("query calls = %d, want 0", query.calls)
	}
}

func TestAuthorStatsServiceReturnsZeroOnQueryError(t *testing.T) {
	t.Parallel()

	service := NewAuthorStatsService(
		&authorStatsQueryStub{err: errors.New("database unavailable")},
		&StatsService{logger: zap.NewNop()},
		&authorStatsCacheStub{},
	)

	if got := service.AggregateByAuthor(context.Background(), "author"); got != (UserVideoStats{}) {
		t.Fatalf("AggregateByAuthor() = %#v, want zero", got)
	}
}
