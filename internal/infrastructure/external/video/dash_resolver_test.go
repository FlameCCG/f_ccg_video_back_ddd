package video_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	infra "f_ccg_video_back_ddd/internal/infrastructure/external/video"
	"go.uber.org/zap"
)

func TestHTTPDASHVariantResolver(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"valid", 200, `<MPD><Period><AdaptationSet contentType="video"><Representation id="v" bandwidth="100000" width="1280" height="720"><BaseURL>video.mp4</BaseURL></Representation></AdaptationSet></Period></MPD>`, false},
		{"upstream failure", 503, "unavailable", true},
		{"invalid xml", 200, "<MPD>", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("token") != "test" || r.Header.Get("Accept") == "" {
					t.Error("query or Accept header lost")
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			variants, err := infra.NewHTTPDASHVariantResolver(zap.NewNop()).Variants(t.Context(), 1, server.URL+"/manifest.mpd?token=test")
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr && (len(variants) != 1 || variants[0].Height != 720) {
				t.Fatalf("variants=%+v", variants)
			}
		})
	}
}

func TestHTTPDASHVariantResolverRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := infra.NewHTTPDASHVariantResolver(zap.NewNop()).Variants(ctx, 1, "http://127.0.0.1/manifest.mpd")
	if err == nil {
		t.Fatal("cancelled request succeeded")
	}
}
