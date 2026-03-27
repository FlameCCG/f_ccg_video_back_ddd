package search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/search"
	es "github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

func TestKnnRequestUsesRawCosineThreshold(t *testing.T) {
	t.Parallel()
	for _, configured := range []float64{0, 0.6} {
		t.Run(strconv.FormatFloat(configured, 'f', 1, 64), func(t *testing.T) {
			t.Parallel()
			requests := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				requests <- body
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"hits":{"hits":[]}}`))
			}))
			defer server.Close()
			client, err := es.NewClient(es.Config{Addresses: []string{server.URL}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewVideoVectorIndex(client, zap.NewNop()).runKnnSearch(t.Context(), app.VectorConfig{Index: "test", MinSimilarity: configured}, []float32{1, 0}, 3)
			if err != nil {
				t.Fatal(err)
			}
			knn := (<-requests)["knn"].(map[string]any)
			threshold, present := knn["similarity"]
			if configured == 0 && present || configured > 0 && threshold != configured {
				t.Fatalf("configured=%v knn.similarity=%v present=%v", configured, threshold, present)
			}
		})
	}
}
