package oauth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithRetryRetriesTimeoutOnce(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			time.Sleep(150 * time.Millisecond)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 100 * time.Millisecond}

	resp, err := DoWithRetry(client, func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	})
	if err != nil {
		t.Fatalf("expected retry success, got err: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
}

func TestDoWithRetryDoesNotRetryClientError(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}

	resp, err := DoWithRetry(client, func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if attempts.Load() != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts.Load())
	}
}

func TestDoWithRetryReturnsBuildError(t *testing.T) {
	expectedErr := errors.New("build failed")

	resp, err := DoWithRetry(&http.Client{Timeout: time.Second}, func() (*http.Request, error) {
		return nil, expectedErr
	})
	if resp != nil {
		_ = resp.Body.Close()
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected build error, got %v", err)
	}
}
