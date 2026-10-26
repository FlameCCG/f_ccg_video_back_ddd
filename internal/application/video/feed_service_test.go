package video

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

type feedHistoryStub struct {
	recent func(context.Context, uint, int) ([]uint, error)
}

func (feedHistoryStub) Upsert(context.Context, uint, uint, int, int) error { return nil }

func (feedHistoryStub) Find(context.Context, uint, uint) (*domain.PlayHistory, error) {
	return nil, nil
}

func (feedHistoryStub) DeleteByVideoIDs(context.Context, uint, []uint) error { return nil }

func (s feedHistoryStub) RecentVideoIDs(ctx context.Context, userID uint, limit int) ([]uint, error) {
	if s.recent == nil {
		return nil, nil
	}

	return s.recent(ctx, userID, limit)
}

func (feedHistoryStub) ProgressByVideoIDs(context.Context, uint, []uint) map[uint]int { return nil }

type feedQueryStub struct {
	preferred func(context.Context, uint) ([]uint, error)
	fetch     func(context.Context, []uint, []uint, int, int) ([]FeedItem, int64, error)
}

func (s feedQueryStub) PreferredPartitionIDs(ctx context.Context, userID uint) ([]uint, error) {
	if s.preferred == nil {
		return nil, nil
	}

	return s.preferred(ctx, userID)
}

func (s feedQueryStub) FetchFeed(
	ctx context.Context,
	partitionIDs, excludeVideoIDs []uint,
	page, pageSize int,
) ([]FeedItem, int64, error) {
	if s.fetch == nil {
		return []FeedItem{}, 0, nil
	}

	return s.fetch(ctx, partitionIDs, excludeVideoIDs, page, pageSize)
}

func (feedQueryStub) FetchByIDs(context.Context, []uint) (map[uint]FeedItem, error) {
	return map[uint]FeedItem{}, nil
}

func (feedQueryStub) FetchHotFromDB(context.Context, int, int) ([]FeedItem, int64, error) {
	return nil, 0, nil
}

func (feedQueryStub) CountPublished(context.Context) (int64, error) { return 0, nil }

func (feedQueryStub) FetchRankFromDB(context.Context, int, int) ([]FeedItem, error) {
	return nil, nil
}

func TestHomepageFeedLoadsHistoryAndPreferencesConcurrently(t *testing.T) {
	t.Parallel()

	started := make(chan string, 2)
	release := make(chan struct{})
	load := func(name string) func(context.Context, uint) ([]uint, error) {
		return func(ctx context.Context, _ uint) ([]uint, error) {
			started <- name

			select {
			case <-release:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	service := NewFeedService(
		feedQueryStub{preferred: load("preferences")},
		feedHistoryStub{recent: func(ctx context.Context, _ uint, _ int) ([]uint, error) {
			return load("history")(ctx, 0)
		}},
		nil,
		nil,
	)

	done := make(chan error, 1)

	go func() {
		_, _, err := service.HomepageFeed(t.Context(), 1, 0, 1, 20)
		done <- err
	}()

	seen := map[string]bool{}

	for range 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatal("history and preference queries did not start concurrently")
		}
	}

	close(release)

	{
		err := <-done
		require.NoError(t, err, "HomepageFeed: %v", err)
	}

	require.True(t, seen["history"], "started queries = %v", seen)
	require.True(t, seen["preferences"], "started queries = %v", seen)
}

func TestHomepageFeedPropagatesSourceErrors(t *testing.T) {
	t.Parallel()

	historyErr := errors.New("history unavailable")
	service := NewFeedService(
		feedQueryStub{},
		feedHistoryStub{recent: func(context.Context, uint, int) ([]uint, error) {
			return nil, historyErr
		}},
		nil,
		nil,
	)

	{
		_, _, err := service.HomepageFeed(t.Context(), 1, 9, 1, 20)
		require.ErrorIs(t, err, historyErr, "history error = %v, want %v", err, historyErr)
	}

	preferenceErr := errors.New("preferences unavailable")
	service = NewFeedService(
		feedQueryStub{preferred: func(context.Context, uint) ([]uint, error) {
			return nil, preferenceErr
		}},
		feedHistoryStub{},
		nil,
		nil,
	)

	{
		_, _, err := service.HomepageFeed(t.Context(), 1, 0, 1, 20)
		require.ErrorIs(t, err, preferenceErr, "preference error = %v, want %v", err, preferenceErr)
	}
}

func TestHomepageFeedPropagatesCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := NewFeedService(
		feedQueryStub{preferred: func(ctx context.Context, _ uint) ([]uint, error) {
			return nil, ctx.Err()
		}},
		feedHistoryStub{recent: func(ctx context.Context, _ uint, _ int) ([]uint, error) {
			return nil, ctx.Err()
		}},
		nil,
		nil,
	)

	{
		_, _, err := service.HomepageFeed(ctx, 1, 0, 1, 20)
		require.ErrorIs(t, err, context.Canceled, "error = %v, want context.Canceled", err)
	}
}
