package contentops

import (
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestTranscodeLocalConsumerConfigRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	{
		err := os.WriteFile(path, []byte("transcode:\n  cpuMode: true\n"), 0o600)
		require.NoError(t, err)
	}

	handler, provider := newConfigHandler(t, &config.Config{Transcode: &config.Transcode{CPUMode: true}}, path)
	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
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
			ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/admin/site/config?name=transcode", strings.NewReader(step.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			handler.AdminUpdateConfigItemView(ctx)

			var response struct {
				Code int `json:"code"`
			}
			{
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.False(t, err != nil || response.Code != 0, "update failed: %s", rec.Body.String())
			}

			require.Equal(t, step.want, provider.Get().Transcode.LocalConsumerDisabled, "incorrect published local consumer role")

			v := viper.New()
			v.SetConfigFile(path)

			{
				err := v.ReadInConfig()
				require.NoError(t, err)
			}

			var persisted config.Config
			{
				err := v.Unmarshal(&persisted)
				require.NoError(t, err)
			}

			require.NotNil(t, persisted.Transcode, "local consumer role was not persisted")
			require.Equal(t, step.want, persisted.Transcode.LocalConsumerDisabled, "local consumer role was not persisted")

			rec = httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/admin/site/config?name=transcode", nil)
			handler.AdminConfigItemView(ctx)

			var result struct {
				Data config.Transcode `json:"data"`
			}
			{
				err := json.Unmarshal(rec.Body.Bytes(), &result)
				require.NoError(t, err)
			}

			require.Equal(t, step.want, result.Data.LocalConsumerDisabled, "GET did not return local consumer role: %s", rec.Body.String())
		})
	}
}
