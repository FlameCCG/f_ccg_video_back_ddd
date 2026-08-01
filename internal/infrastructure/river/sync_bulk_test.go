package river

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"f_ccg_video_back_ddd/pkg/constants"

	elastic7 "github.com/olivere/elastic/v7"
	"github.com/siddontang/go-mysql-elasticsearch/elastic"
)

func TestRiverDoBulkWithRetryRetriesTransientFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, `{"error":{"type":"es_rejected_execution_exception","reason":"busy"},"status":503}`)

			return
		}

		writeBulkSuccess(t, w)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	if err := r.doBulkWithRetry(testUserBulkRequests()); err != nil {
		t.Fatalf("doBulkWithRetry failed: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("bulk calls = %d, want 2", got)
	}
}

func TestRiverDoBulkWithRetryRejectsPermanentItemFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"took": 1,
			"errors": true,
			"items": [{
				"index": {
					"_index": "search_users",
					"_id": "users:2",
					"status": 429,
					"error": {
						"type": "es_rejected_execution_exception",
						"reason": "busy"
					}
				}
			}, {
				"index": {
					"_index": "search_users",
					"_id": "users:1",
					"status": 400,
					"error": {
						"type": "mapper_parsing_exception",
						"reason": "invalid field"
					}
				}
			}]
		}`)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	if err := r.doBulkWithRetry(testUserBulkRequests()); err == nil {
		t.Fatal("doBulkWithRetry error = nil, want permanent item error")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("bulk calls = %d, want 1", got)
	}
}

func TestRiverDoBulkWithRetryRetriesTransientItemFailure(t *testing.T) {
	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			_, _ = fmt.Fprint(w, `{
				"took": 1,
				"errors": true,
				"items": [{
					"index": {
						"_index": "search_users",
						"_id": "users:1",
						"status": 429,
						"error": {
							"type": "es_rejected_execution_exception",
							"reason": "busy"
						}
					}
				}]
			}`)

			return
		}

		writeBulkSuccess(t, w)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	if err := r.doBulkWithRetry(testUserBulkRequests()); err != nil {
		t.Fatalf("doBulkWithRetry failed: %v", err)
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("bulk calls = %d, want 2", got)
	}
}

func TestRiverDoBulkWithRetryStopsDuringBackoff(t *testing.T) {
	firstCall := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-firstCall:
		default:
			close(firstCall)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"error":{"type":"es_rejected_execution_exception","reason":"busy"},"status":503}`)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	result := make(chan error, 1)
	go func() {
		result <- r.doBulkWithRetry(testUserBulkRequests())
	}()

	select {
	case <-firstCall:
	case <-time.After(time.Second):
		t.Fatal("bulk request did not start")
	}

	r.cancel()

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("doBulkWithRetry error = nil after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("doBulkWithRetry did not stop after cancellation")
	}
}

func TestIsRetryableESStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "bad request", status: http.StatusBadRequest, want: false},
		{name: "conflict", status: http.StatusConflict, want: false},
		{name: "request timeout", status: http.StatusRequestTimeout, want: true},
		{name: "too early", status: http.StatusTooEarly, want: true},
		{name: "too many requests", status: http.StatusTooManyRequests, want: true},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableESStatus(tt.status); got != tt.want {
				t.Fatalf("isRetryableESStatus(%d) = %t, want %t", tt.status, got, tt.want)
			}
		})
	}
}

func newTestRiver(t *testing.T, server *httptest.Server) *River {
	t.Helper()

	client, err := elastic7.NewClient(
		elastic7.SetURL(server.URL),
		elastic7.SetHttpClient(server.Client()),
		elastic7.SetSniff(false),
		elastic7.SetHealthcheck(false),
	)
	if err != nil {
		t.Fatalf("create ES client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		client.Stop()
	})

	return &River{ctx: ctx, cancel: cancel, es: client}
}

func testUserBulkRequests() []*elastic.BulkRequest {
	return []*elastic.BulkRequest{
		{
			Action: elastic.ActionIndex,
			Index:  constants.UserSearchIndex,
			ID:     "users:1",
			Data: map[string]any{
				"id":       uint(1),
				"username": "tester",
			},
		},
	}
}

func writeBulkSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{
		"took": 1,
		"errors": false,
		"items": [{
			"index": {
				"_index": "search_users",
				"_id": "users:1",
				"status": 201
			}
		}]
	}`)
}
