package oauth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDoWithRetryRetriesTimeoutOnce(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err, "expected retry success, got err: %v", err)

	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected status code: %d", resp.StatusCode)

	require.EqualValues(t, 2, attempts.Load(), "expected 2 attempts, got %d", attempts.Load())
}

func TestDoWithRetryDoesNotRetryClientError(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, err, "unexpected err: %v", err)

	defer func() { _ = resp.Body.Close() }()

	require.EqualValues(t, 1, attempts.Load(), "expected 1 attempt, got %d", attempts.Load())
}

func TestDoWithRetryReturnsBuildError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("build failed")

	resp, err := DoWithRetry(&http.Client{Timeout: time.Second}, func() (*http.Request, error) {
		return nil, expectedErr
	})
	if resp != nil {
		_ = resp.Body.Close()
	}

	require.ErrorIs(t, err, expectedErr, "expected build error, got %v", err)
}
