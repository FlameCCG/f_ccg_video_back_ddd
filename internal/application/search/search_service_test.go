package search

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestApplyVideoMeta(t *testing.T) {
	hits := []VideoHit{
		{ID: 1},
		{ID: 2, AuthorUsername: "old-name"},
	}
	createdAt := time.Date(2026, 3, 24, 10, 20, 30, 0, time.UTC)

	applyVideoMeta(hits, map[uint]VideoSearchMeta{
		1: {ID: 1, AuthorUsername: "alice", CreatedAt: createdAt},
		2: {ID: 2, AuthorUsername: "bob", CreatedAt: createdAt},
	})

	if hits[0].AuthorUsername != "alice" {
		t.Fatalf("expected first author alice, got %q", hits[0].AuthorUsername)
	}

	if !hits[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("expected first createdAt %v, got %v", createdAt, hits[0].CreatedAt)
	}

	if hits[1].AuthorUsername != "bob" {
		t.Fatalf("expected second author bob, got %q", hits[1].AuthorUsername)
	}

	if !hits[1].CreatedAt.Equal(createdAt) {
		t.Fatalf("expected second createdAt %v, got %v", createdAt, hits[1].CreatedAt)
	}
}

func TestBuildRelatedTagsDeduplicatesAndCaps(t *testing.T) {
	videos := []VideoHit{
		{Tags: []string{"教程", " 教程 ", ""}},
		{Tags: []string{"vlog"}},
	}

	tags := buildRelatedTags(videos)
	if len(tags) != 2 || tags[0].Name != "教程" || tags[1].Name != "vlog" {
		t.Fatalf("related tags = %+v", tags)
	}

	many := make([]VideoHit, 0, relatedTagLimit+5)
	for i := range relatedTagLimit + 5 {
		many = append(many, VideoHit{Tags: []string{string(rune('a' + i))}})
	}

	if got := buildRelatedTags(many); len(got) != relatedTagLimit {
		t.Fatalf("related tags length = %d, want %d", len(got), relatedTagLimit)
	}
}

func TestSearchFallsBackToDatabaseWhenESUnavailable(t *testing.T) {
	fallback := &fakeSearchFallback{
		available:  true,
		videos:     []VideoHit{{ID: 1, Title: "教程", Tags: []string{"教程"}}},
		videoTotal: 1,
		users:      []UserHit{{ID: 7, Username: "alice"}},
		userTotal:  3,
		countUsers: 3,
	}
	svc := NewSearchService(&fakeKeywordSearch{available: false}, fallback, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "教程", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if result.VideoTotal != 1 || result.UserTotal != 3 || result.TagTotal != 1 {
		t.Fatalf("totals = %+v", result)
	}

	if result.Total != 5 {
		t.Fatalf("total = %d, want 5", result.Total)
	}

	if len(result.Videos) != 1 || len(result.Users) != 1 {
		t.Fatalf("hits = %+v", result)
	}
}

func TestSearchReturnsEmptySlicesForBlankKeywordOnFallback(t *testing.T) {
	svc := NewSearchService(&fakeKeywordSearch{available: false}, &fakeSearchFallback{available: true}, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "   ", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if result.Videos == nil || result.Users == nil {
		t.Fatalf("fallback must return empty (not nil) slices: %+v", result)
	}

	if len(result.Videos) != 0 || len(result.Users) != 0 || result.Total != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSearchPropagatesMultiSearchContractError(t *testing.T) {
	index := &fakeKeywordSearch{available: true, multiErr: ErrUnexpectedMultiSearchSize}
	svc := NewSearchService(index, &fakeSearchFallback{available: true}, zap.NewNop())

	if _, err := svc.Search(context.Background(), Options{Keyword: "x", Page: 1, PageSize: 20}); !errors.Is(err, ErrUnexpectedMultiSearchSize) {
		t.Fatalf("err = %v, want ErrUnexpectedMultiSearchSize", err)
	}
}

func TestSearchHydratesMissingAuthorAndCreatedAt(t *testing.T) {
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
		meta:      []VideoSearchMeta{{ID: 9, AuthorUsername: "alice", CreatedAt: createdAt}},
	}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.Search(context.Background(), Options{Keyword: "t", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if got := result.Videos[0]; got.AuthorUsername != "alice" || !got.CreatedAt.Equal(createdAt) {
		t.Fatalf("hydrated hit = %+v", got)
	}

	if !reflect.DeepEqual(fallback.metaCalls, [][]uint{{9}}) {
		t.Fatalf("hydrate queried ids = %v", fallback.metaCalls)
	}
}

func TestSearchVideosUsesESUserTotalAndSkipsDBCount(t *testing.T) {
	index := &fakeKeywordSearch{
		available: true,
		videos:    VideoHits{Total: 2, Items: []VideoHit{{ID: 1, Title: "a", AuthorUsername: "u", CreatedAt: time.Now()}}},
		userTotal: 42,
	}
	fallback := &fakeSearchFallback{available: true, countUsers: 7}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.SearchVideos(context.Background(), Options{Keyword: "a", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}

	if result.UserTotal != 42 || result.VideoTotal != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSearchVideosFallsBackWhenESFails(t *testing.T) {
	index := &fakeKeywordSearch{available: true, videoErr: errors.New("es down")}
	fallback := &fakeSearchFallback{
		available:  true,
		videos:     []VideoHit{{ID: 5, Title: "b"}},
		videoTotal: 1,
		countUsers: 4,
	}
	svc := NewSearchService(index, fallback, zap.NewNop())

	result, err := svc.SearchVideos(context.Background(), Options{Keyword: "b", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}

	if result.VideoTotal != 1 || result.UserTotal != 4 || len(result.Videos) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSearchUsersNormalizesOptionsBeforeFallback(t *testing.T) {
	fallback := &fakeSearchFallback{
		available: true,
		users:     []UserHit{{ID: 1, Username: "alice"}},
		userTotal: 1,
	}
	svc := NewSearchService(&fakeKeywordSearch{available: false}, fallback, zap.NewNop())

	result, err := svc.SearchUsers(context.Background(), Options{Keyword: "alice"})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	if result.UserTotal != 1 || len(result.Users) != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestUserTotalBestEffortFallsBackToDatabase(t *testing.T) {
	index := &fakeKeywordSearch{available: true, countErr: errors.New("es down")}
	fallback := &fakeSearchFallback{available: true, countUsers: 11}
	svc := NewSearchService(index, fallback, zap.NewNop())

	if got := svc.userTotalBestEffort(context.Background(), "x"); got != 11 {
		t.Fatalf("user total = %d, want 11", got)
	}

	if got := svc.userTotalBestEffort(context.Background(), "  "); got != 0 {
		t.Fatalf("blank keyword total = %d, want 0", got)
	}
}
