package search

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestApplyVideoMeta(t *testing.T) {
	t.Parallel()

	hits := []VideoHit{
		{ID: 1},
		{ID: 2, AuthorUsername: "old-name"},
	}
	createdAt := time.Date(2026, 3, 24, 10, 20, 30, 0, time.UTC)

	applyVideoMeta(hits, map[uint]VideoSearchMeta{
		1: {ID: 1, AuthorUsername: testAlice, CreatedAt: createdAt},
		2: {ID: 2, AuthorUsername: "bob", CreatedAt: createdAt},
	})

	require.Equal(t, testAlice, hits[0].AuthorUsername, "expected first author alice, got %q", hits[0].AuthorUsername)

	require.True(t, hits[0].CreatedAt.Equal(createdAt), "expected first createdAt %v, got %v", createdAt, hits[0].CreatedAt)

	require.Equal(t, "bob", hits[1].AuthorUsername, "expected second author bob, got %q", hits[1].AuthorUsername)

	require.True(t, hits[1].CreatedAt.Equal(createdAt), "expected second createdAt %v, got %v", createdAt, hits[1].CreatedAt)
}

func TestBuildRelatedTagsDeduplicatesAndCaps(t *testing.T) {
	t.Parallel()

	videos := []VideoHit{
		{Tags: []string{"教程", " 教程 ", ""}},
		{Tags: []string{testVlog}},
	}

	tags := buildRelatedTags(videos)
	require.Len(t, tags, 2, "related tags = %+v", tags)
	require.Equal(t, "教程", tags[0].Name, "related tags = %+v", tags)
	require.Equal(t, testVlog, tags[1].Name, "related tags = %+v", tags)

	many := make([]VideoHit, 0, relatedTagLimit+5)
	for i := range relatedTagLimit + 5 {
		many = append(many, VideoHit{Tags: []string{string(rune('a' + i))}})
	}

	{
		got := buildRelatedTags(many)
		require.Len(t, got, relatedTagLimit, "related tags length = %d, want %d", len(got), relatedTagLimit)
	}
}

func TestSearchFallsBackToDatabaseWhenESUnavailable(t *testing.T) {
	t.Parallel()

	fallback := &fakeSearchFallback{
		available:  true,
		videos:     []VideoHit{{ID: 1, Title: "教程", Tags: []string{"教程"}}},
		videoTotal: 1,
		users:      []UserHit{{ID: 7, Username: testAlice}},
		userTotal:  3,
		countUsers: 3,
	}
	svc := NewSearchService(&fakeKeywordSearch{available: false}, fallback, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "教程", Page: 1, PageSize: 20})
	require.NoError(t, err, "Search: %v", err)

	require.EqualValues(t, 1, result.VideoTotal, "totals = %+v", result)
	require.EqualValues(t, 3, result.UserTotal, "totals = %+v", result)
	require.EqualValues(t, 1, result.TagTotal, "totals = %+v", result)

	require.EqualValues(t, 5, result.Total, "total = %d, want 5", result.Total)

	require.Len(t, result.Videos, 1, "hits = %+v", result)
	require.Len(t, result.Users, 1, "hits = %+v", result)
}

func TestSearchReturnsEmptySlicesForBlankKeywordOnFallback(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(&fakeKeywordSearch{available: false}, &fakeSearchFallback{available: true}, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "   ", Page: 1, PageSize: 20})
	require.NoError(t, err, "Search: %v", err)

	require.NotNil(t, result.Videos, "fallback must return empty (not nil) slices: %+v", result)
	require.NotNil(t, result.Users, "fallback must return empty (not nil) slices: %+v", result)

	require.Empty(t, result.Videos, "unexpected result: %+v", result)
	require.Empty(t, result.Users, "unexpected result: %+v", result)
	require.EqualValues(t, 0, result.Total, "unexpected result: %+v", result)
}

func TestSearchPropagatesMultiSearchContractError(t *testing.T) {
	t.Parallel()

	index := &fakeKeywordSearch{available: true, multiErr: ErrUnexpectedMultiSearchSize}
	svc := NewSearchService(index, &fakeSearchFallback{available: true}, zap.NewNop())

	{
		_, err := svc.Search(context.Background(), Options{Keyword: "x", Page: 1, PageSize: 20})
		require.ErrorIs(t, err, ErrUnexpectedMultiSearchSize, "err = %v, want ErrUnexpectedMultiSearchSize", err)
	}
}

func TestSearchHydratesMissingAuthorAndCreatedAt(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 3, 24, 10, 20, 30, 0, time.UTC)
	index := &fakeKeywordSearch{
		available: true,
		multi: CombinedHits{
			Videos: VideoHits{Total: 1, Items: []VideoHit{{ID: 9, Title: "t"}}},
			Users:  UserHits{},
		},
	}
	fallback := &fakeSearchFallback{
		available: true,
		meta:      []VideoSearchMeta{{ID: 9, AuthorUsername: testAlice, CreatedAt: createdAt}},
	}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "t", Page: 1, PageSize: 20})
	require.NoError(t, err, "Search: %v", err)

	{
		got := result.Videos[0]
		require.Equal(t, testAlice, got.AuthorUsername, "hydrated hit = %+v", got)
		require.True(t, got.CreatedAt.Equal(createdAt), "hydrated hit = %+v", got)
	}

	require.True(t, reflect.DeepEqual(fallback.metaCalls, [][]uint{{9}}), "hydrate queried ids = %v", fallback.metaCalls)
}

func TestSearchVideosUsesESUserTotalAndSkipsDBCount(t *testing.T) {
	t.Parallel()

	index := &fakeKeywordSearch{
		available: true,
		videos:    VideoHits{Total: 2, Items: []VideoHit{{ID: 1, Title: "a", AuthorUsername: "u", CreatedAt: time.Now()}}},
		userTotal: 42,
	}
	fallback := &fakeSearchFallback{available: true, countUsers: 7}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.SearchVideos(context.Background(), Options{Keyword: "a", Page: 1, PageSize: 20})
	require.NoError(t, err, "SearchVideos: %v", err)

	require.EqualValues(t, 42, result.UserTotal, "result = %+v", result)
	require.EqualValues(t, 2, result.VideoTotal, "result = %+v", result)
}

func TestSearchVideosFallsBackWhenESFails(t *testing.T) {
	t.Parallel()

	index := &fakeKeywordSearch{available: true, videoErr: errors.New("es down")}
	fallback := &fakeSearchFallback{
		available:  true,
		videos:     []VideoHit{{ID: 5, Title: "b"}},
		videoTotal: 1,
		countUsers: 4,
	}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.SearchVideos(context.Background(), Options{Keyword: "b", Page: 1, PageSize: 20})
	require.NoError(t, err, "SearchVideos: %v", err)

	require.EqualValues(t, 1, result.VideoTotal, "result = %+v", result)
	require.EqualValues(t, 4, result.UserTotal, "result = %+v", result)
	require.Len(t, result.Videos, 1, "result = %+v", result)
}

func TestSearchUsersNormalizesOptionsBeforeFallback(t *testing.T) {
	t.Parallel()

	fallback := &fakeSearchFallback{
		available: true,
		users:     []UserHit{{ID: 1, Username: testAlice}},
		userTotal: 1,
	}
	svc := NewSearchService(&fakeKeywordSearch{available: false}, fallback, zap.NewNop())

	result, err := svc.SearchUsers(context.Background(), Options{Keyword: testAlice})
	require.NoError(t, err, "SearchUsers: %v", err)

	require.EqualValues(t, 1, result.UserTotal, "result = %+v", result)
	require.Len(t, result.Users, 1, "result = %+v", result)
}

func TestUserTotalBestEffortFallsBackToDatabase(t *testing.T) {
	t.Parallel()

	index := &fakeKeywordSearch{available: true, countErr: errors.New("es down")}
	fallback := &fakeSearchFallback{available: true, countUsers: 11}
	svc := NewSearchService(index, fallback, zap.NewNop())

	{
		got := svc.userTotalBestEffort(context.Background(), "x")
		require.EqualValues(t, 11, got, "user total = %d, want 11", got)
	}

	{
		got := svc.userTotalBestEffort(context.Background(), "  ")
		require.EqualValues(t, 0, got, "blank keyword total = %d, want 0", got)
	}
}
