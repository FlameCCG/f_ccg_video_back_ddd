package ark

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		response.Body.Close()
	}
	if !errors.Is(err, domain.ErrAssetHostNotAllowed) {
		t.Fatalf("Open() error = %v, want rejected redirect", err)
	}
	if reached.Load() {
		t.Fatal("proxy followed redirect to a disallowed host")
	}
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
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			err = validateAssetRedirect(req, make([]*http.Request, tc.redirects))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
