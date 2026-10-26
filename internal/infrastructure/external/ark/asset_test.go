package ark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"

	domain "f_ccg_video_back_ddd/internal/domain/ai"
)

func TestAssetProxyRejectsRedirectOutsideAllowlist(t *testing.T) {
	t.Parallel()

	var reached atomic.Bool

	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached.Store(true)
	}))
	t.Cleanup(target.Close)

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(source.Close)

	response, err := NewAssetProxy().Open(context.Background(), aiapp.RuntimeConfig{}, source.URL, nil)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}

	require.ErrorIs(t, err, domain.ErrAssetHostNotAllowed, "Open() error = %v, want rejected redirect", err)

	require.False(t, reached.Load(), "proxy followed redirect to a disallowed host")
}

func TestValidateAssetRedirect(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		url       string
		redirects int
		wantErr   bool
	}{
		{name: "allowed CDN", url: "https://image.byteimg.com/result.png"},
		{name: "lookalike domain", url: "https://byteimg.com.example.org/result.png", wantErr: true},
		{name: "unsupported scheme", url: "file:///etc/passwd", wantErr: true},
		{name: "redirect loop", url: "https://image.byteimg.com/result.png", redirects: 10, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			err = validateAssetRedirect(req, make([]*http.Request, tc.redirects))
			require.Equal(t, tc.wantErr, (err != nil), "error = %v, wantErr %v", err, tc.wantErr)
		})
	}
}
