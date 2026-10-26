package search

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/search"

	"github.com/olivere/elastic/v7"
)

func TestUserIndexerBulkIndexUsersDoesNotForceRefresh(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(r.Context())

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
	}))
	defer server.Close()

	client, err := elastic.NewClient(
		elastic.SetURL(server.URL),
		elastic.SetHttpClient(server.Client()),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
	)
	require.NoError(t, err, "create ES client: %v", err)

	defer client.Stop()

	indexer := NewUserIndexer(client)

	err = indexer.BulkIndexUsers(context.Background(), []app.UserSearchDocument{
		{ID: 1, Username: "tester"},
	})
	require.NoError(t, err, "BulkIndexUsers failed: %v", err)

	select {
	case req := <-requests:
		{
			got := req.URL.Query().Get("refresh")
			require.Empty(t, got, "refresh query = %q, want empty", got)
		}
	default:
		t.Fatal("bulk request was not captured")
	}
}
