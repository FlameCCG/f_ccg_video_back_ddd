package search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

// 验证实际联想请求的可见性过滤。
func TestFieldSuggestFiltersPublishedDocuments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		docType       string
		publishedOnly bool
	}{
		{searchdomain.DocTypeVideo, true},
		{searchdomain.DocTypeTag, true},
		{searchdomain.DocTypeUser, false},
	} {
		t.Run(test.docType, func(t *testing.T) {
			t.Parallel()
			requests := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- body
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_id":"1","_source":{"title":"demo","tags":["demo"],"username":"demo"}}]}}`)
			}))
			defer server.Close()
			client, err := elastic.NewClient(elastic.SetURL(server.URL), elastic.SetSniff(false), elastic.SetHealthcheck(false))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Stop()
			items, err := NewAdapter(client, zap.NewNop()).FieldSuggest(t.Context(), test.docType, "dem", 5)
			if err != nil || len(items) != 1 || items[0].Value != "demo" {
				t.Fatalf("suggestion response: items=%+v err=%v", items, err)
			}
			request := <-requests
			query, _ := request["query"].(map[string]any)
			boolean, _ := query["bool"].(map[string]any)
			filters, ok := boolean["filter"].([]any)
			if !ok && boolean["filter"] != nil {
				filters = []any{boolean["filter"]}
			}
			publishedOnly := false
			for _, filter := range filters {
				object, _ := filter.(map[string]any)
				term, _ := object["term"].(map[string]any)
				value := term["status"]
				if options, ok := value.(map[string]any); ok {
					value = options["value"]
				}
				if value == float64(1) {
					publishedOnly = true
				}
			}
			if publishedOnly != test.publishedOnly {
				t.Fatalf("published-only filter=%v want=%v; request=%+v", publishedOnly, test.publishedOnly, request)
			}
		})
	}
}
