package contentops

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestTranscodeLocalConsumerConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("transcode:\n  cpuMode: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	handler, provider := newConfigHandler(t, &config.Config{Transcode: &config.Transcode{CpuMode: true}}, path)
	for _, step := range []struct {
		name string
		body string
		want bool
	}{
		{"disable", `{"cpuMode":true,"localConsumerDisabled":true}`, true},
		{"legacy update preserves role", `{"cpuMode":true,"maxWorkers":4}`, true},
		{"null preserves role", `{"cpuMode":true,"localConsumerDisabled":null}`, true},
		{"enable", `{"cpuMode":true,"localConsumerDisabled":false}`, false},
	} {
		t.Run(step.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/v1/admin/site/config?name=transcode", strings.NewReader(step.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			handler.AdminUpdateConfigItemView(ctx)
			var response struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil || response.Code != 0 {
				t.Fatalf("update failed: %s", rec.Body.String())
			}
			if provider.Get().Transcode.LocalConsumerDisabled != step.want {
				t.Fatal("incorrect published local consumer role")
			}
			v := viper.New()
			v.SetConfigFile(path)
			if err := v.ReadInConfig(); err != nil {
				t.Fatal(err)
			}
			var persisted config.Config
			if err := v.Unmarshal(&persisted); err != nil {
				t.Fatal(err)
			}
			if persisted.Transcode == nil || persisted.Transcode.LocalConsumerDisabled != step.want {
				t.Fatal("local consumer role was not persisted")
			}
			rec = httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/admin/site/config?name=transcode", nil)
			handler.AdminConfigItemView(ctx)
			var result struct {
				Data config.Transcode `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Data.LocalConsumerDisabled != step.want {
				t.Fatalf("GET did not return local consumer role: %s", rec.Body.String())
			}
		})
	}
}
