package bootstrap

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/validate"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRouterRouteListerUsesRealRouter(t *testing.T) {
	t.Parallel()

	provider := infraconfig.NewProvider(&config.Config{
		Server: &config.Server{Mode: gin.TestMode},
	})
	httpRouter := NewCLIRouter(provider, nil, zap.NewNop(), validate.New())
	routes := NewRouterRouteLister(httpRouter).Routes()

	require.NotEmpty(t, routes, "real Router returned no routes")

	got := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		got[route.Method+" "+route.Path] = struct{}{}
	}

	for _, want := range []string{
		"POST /v1/admin/rbac/menu/generate",
		"PUT /v1/admin/rbac/menu/move",
		"POST /v1/admin/video/vector/rebuild",
		"GET /v1/admin/video/vector/status",
	} {
		{
			_, ok := got[want]
			assert.True(t, ok, "real Router route list is missing %s", want)
		}
	}
}
