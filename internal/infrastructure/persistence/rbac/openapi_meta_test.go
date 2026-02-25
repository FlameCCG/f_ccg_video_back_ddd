package rbac_test

import (
	"os"
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
)

func TestOpenAPIMetadataCoversRegisteredAPIRoutes(t *testing.T) {
	t.Chdir("../../../..")
	meta, err := infra.NewOpenAPIMetaLoader().Load()
	if err != nil {
		t.Fatalf("load OpenAPI metadata: %v", err)
	}
	raw, err := os.ReadFile("internal/interfaces/http/router/testdata/routes_golden.txt")
	if err != nil {
		t.Fatalf("read registered routes: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "/v1/") || !domain.IsResourceMethod(fields[0]) {
			continue
		}
		t.Run(fields[0]+" "+fields[1], func(t *testing.T) {
			item := domain.LookupResourceMeta(meta, fields[0], fields[1])
			if strings.TrimSpace(item.Summary) == "" || len(item.Tags) == 0 {
				t.Errorf("API resource must have an OpenAPI summary and tags: %+v", item)
			}
		})
	}
}
