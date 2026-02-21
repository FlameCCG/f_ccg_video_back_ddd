package bootstrap

import (
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRouterRouteListerUsesRealRouter(t *testing.T) {
	provider := config.NewProvider(&config.Config{
		Server: &config.Server{Mode: gin.TestMode},
	})
	httpRouter := NewCLIRouter(provider, nil, zap.NewNop(), validate.New())
	routes := NewRouterRouteLister(httpRouter).Routes()

	if len(routes) == 0 {
		t.Fatal("real Router returned no routes")
	}

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
		if _, ok := got[want]; !ok {
			t.Errorf("real Router route list is missing %s", want)
		}
	}
}
