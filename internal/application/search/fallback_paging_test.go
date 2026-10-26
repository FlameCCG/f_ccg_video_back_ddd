package search

import (
	"context"
	"go.uber.org/zap"
	"testing"
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
		if err != nil {
			t.Fatal(err)
		}
		if len(fallback.options) == 0 {
			t.Fatal("fallback not reached")
		}
		for _, opts := range fallback.options {
			if opts.Page != 1 || opts.PageSize != 20 {
				t.Fatalf("unnormalized fallback options: %+v", opts)
			}
		}
	}
}
