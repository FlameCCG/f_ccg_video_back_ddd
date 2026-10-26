package rbac_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
)

//nolint:paralleltest // This test changes process environment or working directory.
func TestOpenAPIMetadataCoversRegisteredAPIRoutes(t *testing.T) {
	t.Chdir("../../../..")

	meta, err := infra.NewOpenAPIMetaLoader().Load()
	require.NoError(t, err, "load OpenAPI metadata: %v", err)

	raw, err := os.ReadFile("internal/interfaces/http/router/testdata/routes_golden.txt")
	require.NoError(t, err, "read registered routes: %v", err)

	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[1], "/v1/") || !domain.IsResourceMethod(fields[0]) {
			continue
		}

		//nolint:paralleltest // Subtests exercise a shared parent fixture in sequence.
		t.Run(fields[0]+" "+fields[1], func(t *testing.T) {
			item := domain.LookupResourceMeta(meta, fields[0], fields[1])
			assert.False(t, strings.TrimSpace(item.Summary) == "" || len(item.Tags) == 0, "API resource must have an OpenAPI summary and tags: %+v", item)
		})
	}
}
