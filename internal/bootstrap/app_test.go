package bootstrap

import (
	"f_ccg_video_back_ddd/internal/config"
	"strings"
	"testing"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	httprouter "f_ccg_video_back_ddd/internal/interfaces/http/router"

	"github.com/gin-gonic/gin"
)

func TestAppServeReturnsListenError(t *testing.T) {
	t.Parallel()

	provider := infraconfig.NewProvider(&config.Config{Server: &config.Server{
		Addr: "invalid-address-without-port",
		Mode: gin.TestMode,
	}})
	router := httprouter.New(
		provider,
		nil,
		nil,
		nil, nil, nil,
		nil,
		nil,
		nil,
		nil, nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	err := (&App{cfgProvider: provider, router: router}).Serve()
	if err == nil || !strings.Contains(err.Error(), "run HTTP server") {
		t.Fatalf("Serve() error = %v, want listen error", err)
	}
}
