package search

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type capturingFallback struct {
	fakeSearchFallback
	options []Options
}

func (f *capturingFallback) SearchVideos(_ context.Context, opts Options) ([]VideoHit, int64, error) {
	f.options = append(f.options, opts)
	return nil, 0, nil
}

func (f *capturingFallback) SearchUsers(_ context.Context, opts Options) ([]UserHit, int64, error) {
	f.options = append(f.options, opts)
	return nil, 0, nil
}

func TestUnavailableSearchNormalizesBeforeFallback(t *testing.T) {
	t.Parallel()

	for _, videoOnly := range []bool{false, true} {
		fallback := &capturingFallback{fakeSearchFallback: fakeSearchFallback{available: true}}
		svc := NewSearchService(&fakeKeywordSearch{}, fallback, zap.NewNop())
		opts := Options{Keyword: "query", Page: -4, PageSize: -2}

		var err error
		if videoOnly {
			_, err = svc.SearchVideos(context.Background(), opts)
		} else {
			_, err = svc.Search(context.Background(), opts)
		}

		require.NoError(t, err)

		require.NotEmpty(t, fallback.options, "fallback not reached")

		for _, opts := range fallback.options {
			require.Equal(t, 1, opts.Page, "unnormalized fallback options: %+v", opts)
			require.Equal(t, 20, opts.PageSize, "unnormalized fallback options: %+v", opts)
		}
	}
}
