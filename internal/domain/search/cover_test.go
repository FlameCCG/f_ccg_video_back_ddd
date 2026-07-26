package search

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsDataURI(t *testing.T) {
	if !IsDataURI(" DATA:image/png;base64,AAA ") {
		t.Fatal("data URI should be detected case-insensitively")
	}

	if IsDataURI("https://example.com/a.png") {
		t.Fatal("http url is not a data URI")
	}
}

func TestJoinAssetBaseURL(t *testing.T) {
	if got := JoinAssetBaseURL("https://cdn.example.com/", "uploads\\a.png"); got != "https://cdn.example.com/uploads/a.png" {
		t.Fatalf("joined = %q", got)
	}

	if got := JoinAssetBaseURL("  ", "uploads/a.png"); got != "" {
		t.Fatalf("empty base should yield empty string, got %q", got)
	}
}

func TestIsPubliclyReachableHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "cdn.example.com", want: true},
		{host: "203.0.113.10", want: true},
		{host: "minio", want: false},
		{host: "localhost", want: false},
		{host: "svc.local", want: false},
		{host: "svc.internal", want: false},
		{host: "127.0.0.1", want: false},
		{host: "10.0.0.5", want: false},
		{host: "192.168.1.7", want: false},
		{host: "169.254.1.1", want: false},
		{host: "0.0.0.0", want: false},
		{host: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := IsPubliclyReachableHost(tt.host); got != tt.want {
				t.Fatalf("IsPubliclyReachableHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestSafeJoinUnderRootDeduplicatesRootPrefix(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")

	got, err := SafeJoinUnderRoot(root, "uploads/covers/a.png")
	if err != nil {
		t.Fatalf("SafeJoinUnderRoot: %v", err)
	}

	want := filepath.Join(root, "covers", "a.png")
	if got != want {
		t.Fatalf("joined = %q, want %q", got, want)
	}

	got, err = SafeJoinUnderRoot(root, "covers/b.png")
	if err != nil {
		t.Fatalf("SafeJoinUnderRoot: %v", err)
	}

	if want := filepath.Join(root, "covers", "b.png"); got != want {
		t.Fatalf("joined = %q, want %q", got, want)
	}
}

func TestSafeJoinUnderRootRejectsEscape(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")

	for _, relative := range []string{"../secret.png", "covers/../../secret.png"} {
		if _, err := SafeJoinUnderRoot(root, relative); err == nil {
			t.Fatalf("expected escape rejection for %q", relative)
		} else if !errors.Is(err, ErrCoverUnavailable) {
			t.Fatalf("error should wrap ErrCoverUnavailable, got %v", err)
		} else if !strings.Contains(err.Error(), "escapes the storage root") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
