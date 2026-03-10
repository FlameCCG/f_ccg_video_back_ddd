package search

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngPixel = []byte{
	0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
	0x89,
}

func TestIsPubliclyReachableHost(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"cdn.example.com": true,
		"203.0.113.10":    true,
		"localhost":       false,
		"minio":           false,
		"127.0.0.1":       false,
		"10.0.0.5":        false,
		"192.168.1.20":    false,
		"172.16.3.4":      false,
		"storage.local":   false,
		"":                false,
	}

	for host, want := range tests {
		if got := isPubliclyReachableHost(host); got != want {
			t.Fatalf("isPubliclyReachableHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestResolveCoverEmbedSourcePassesThroughDataURI(t *testing.T) {
	t.Parallel()

	const uri = "data:image/png;base64,AAAA"
	got, err := resolveCoverEmbedSource(context.Background(), http.DefaultClient, uri, "", "")
	if err != nil {
		t.Fatalf("resolveCoverEmbedSource() error = %v", err)
	}
	if got != uri {
		t.Fatalf("resolveCoverEmbedSource() = %q, want %q", got, uri)
	}
}

func TestResolveCoverEmbedSourceKeepsPublicURL(t *testing.T) {
	t.Parallel()

	const cover = "https://cdn.example.com/covers/1.jpg"
	got, err := resolveCoverEmbedSource(context.Background(), http.DefaultClient, cover, "", "")
	if err != nil {
		t.Fatalf("resolveCoverEmbedSource() error = %v", err)
	}
	if got != cover {
		t.Fatalf("public URL changed: got %q", got)
	}
}

func TestResolveCoverEmbedSourceRejectsPrivateHostWithoutFetching(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "http://127.0.0.1/private", http.StatusFound)
	}))
	defer server.Close()
	_, err := resolveCoverEmbedSource(context.Background(), server.Client(), server.URL+"/cover.png", "", "")
	if !errors.Is(err, errCoverUnavailable) || calls != 0 {
		t.Fatalf("private fetch: calls=%d err=%v", calls, err)
	}
}

func TestResolveCoverEmbedSourceRejectsNonImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html>not found</html>"))
	}))
	t.Cleanup(server.Close)

	_, err := resolveCoverEmbedSource(context.Background(), server.Client(), server.URL, "", "")
	if !errors.Is(err, errCoverUnavailable) {
		t.Fatalf("error = %v, want errCoverUnavailable", err)
	}
}

func TestResolveCoverEmbedSourceRejectsEmptyCover(t *testing.T) {
	t.Parallel()

	if _, err := resolveCoverEmbedSource(
		context.Background(), http.DefaultClient, "   ", "", "",
	); !errors.Is(err, errCoverUnavailable) {
		t.Fatalf("error = %v, want errCoverUnavailable", err)
	}
}

func TestJoinAssetBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		base     string
		relative string
		want     string
	}{
		{base: "https://cdn.example.com/", relative: "/uploads/a.jpg", want: "https://cdn.example.com/uploads/a.jpg"},
		{base: "https://cdn.example.com", relative: "uploads\\a.jpg", want: "https://cdn.example.com/uploads/a.jpg"},
		{base: "", relative: "uploads/a.jpg", want: ""},
	}
	for _, test := range tests {
		if got := joinAssetBaseURL(test.base, test.relative); got != test.want {
			t.Fatalf("joinAssetBaseURL(%q, %q) = %q, want %q", test.base, test.relative, got, test.want)
		}
	}
}

func TestSafeJoinUnderRootBlocksTraversal(t *testing.T) {
	t.Parallel()

	if _, err := safeJoinUnderRoot(t.TempDir(), "../../etc/passwd"); !errors.Is(err, errCoverUnavailable) {
		t.Fatalf("error = %v, want errCoverUnavailable", err)
	}
}

func TestSafeJoinUnderRootDeduplicatesRootPrefix(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "uploads")
	if err := os.MkdirAll(filepath.Join(root, "cover"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := safeJoinUnderRoot(root, "uploads/cover/x.jpg")
	if err != nil {
		t.Fatalf("safeJoinUnderRoot() error = %v", err)
	}
	want := filepath.Join(root, "cover", "x.jpg")
	if got != want {
		t.Fatalf("safeJoinUnderRoot() = %q, want %q", got, want)
	}
}

func TestBuildImageDataURISniffsContentType(t *testing.T) {
	t.Parallel()

	got, err := buildImageDataURI(pngPixel, "application/octet-stream")
	if err != nil {
		t.Fatalf("buildImageDataURI() error = %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("buildImageDataURI() = %q, want png data URI", got)
	}
}

func TestBuildImageDataURIRejectsOversize(t *testing.T) {
	t.Parallel()

	_, err := buildImageDataURI(make([]byte, maxCoverImageBytes+1), "image/png")
	if !errors.Is(err, errCoverUnavailable) {
		t.Fatalf("error = %v, want errCoverUnavailable", err)
	}
}
