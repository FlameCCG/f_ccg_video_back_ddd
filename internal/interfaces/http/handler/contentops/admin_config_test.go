package contentops

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

const (
	testGoogleClientID = "google-client-id"
	testGoogleRedirect = "https://example.com/oauth/google"
)

// newConfigHandler 构造只装配站点配置用例的处理器。
//
// 原测试通过 global.SetConfig / global.ConfigPath 操作全局状态；
// 这里改为注入式：配置快照由 *infraconfig.Provider 持有，配置文件路径注入写回器。
func newConfigHandler(t *testing.T, cfg *config.Config, configPath string) (*Handler, *infraconfig.Provider) {
	t.Helper()

	provider := infraconfig.NewProvider(cfg)
	writer := persistence.NewConfigFileWriter(configPath)

	return NewHandler(nil, nil, app.NewSiteConfigService(provider, writer), nil, nil), provider
}

func TestAdminConfigItemViewReturnsNamedConfig(t *testing.T) {
	t.Parallel()

	handler, provider := newConfigHandler(t, &config.Config{
		Site: &site.Site{
			Login: &site.Login{
				QQLogin: true,
			},
			Storage: &site.Storage{
				ChunkDir: "runtime/chunks",
				Local: &site.Local{
					Enable: true,
					Path:   "uploads",
				},
				MinIO: &site.MinIO{
					Enable:           true,
					UserUploadPrefix: "user_files",
					SecretKey:        "minio-secret-key",
					Bucket:           "videos",
					PublicPrefixes:   []string{"user_files", "covers"},
				},
			},
		},
		Transcode: &config.Transcode{
			MaxWorkers:           4,
			TranscodeResolutions: []uint16{360, 720, 1080},
			Mp4Enable:            true,
			Threads:              8,
			GpuMode:              true,
		},
		ThirdLogin: &config.ThirdLogin{
			QQ: &thirdlogin.QQ{
				AppID:    "qq-app-id",
				AppKey:   "qq-app-key",
				Redirect: "https://example.com/oauth/qq",
			},
			Google: &thirdlogin.Google{
				ClientID:     testGoogleClientID,
				ClientSecret: "google-client-secret",
				Redirect:     testGoogleRedirect,
			},
		},
		AI: &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
			ChatModelBaseURL: "https://api.deepseek.com/anthropic",
			ChatModelAPIKey:  "deepseek-test-key",
			ChatModel:        testDeepseekV4Flash,
			ChatModels: []config.AIModel{
				{Label: "DeepSeek V4 Flash", Value: testDeepseekV4Flash},
				{Label: "DeepSeek V4 Pro", Value: "deepseek-v4-pro"},
			},
			ThinkingEnabled: true,
			ThinkingEffort:  "high",
			ThinkingEfforts: []config.AIModel{
				{Label: "High", Value: "high"},
				{Label: "Max", Value: "max"},
			},
			EmbeddingModel:      "doubao-embedding-vision-251215",
			EmbeddingDimensions: 2048,
			DoubaoModelAPIKey:   "doubao-test-key",
			SystemPrompt:        "你是一个严谨的多模态助手。",
			ImageModel:          "doubao-seedream-5-0-lite-260128",
			VideoModel:          "doubao-seedance-1-5-pro-251215",
			VectorIndex:         "search_videos_vector",
			VectorTopK:          12,
			TimeoutSec:          60,
		},
		Server: &config.Server{
			Name:        "test-server",
			Addr:        "0.0.0.0:8080",
			Mode:        "debug",
			CorsOrigins: []string{"http://localhost:3000", "https://www.example.com"},
		},
	}, filepath.Join(t.TempDir(), "config.yaml"))

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("site", func(t *testing.T) { checkNamedSiteConfig(t, handler, provider) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("transcode", func(t *testing.T) { checkNamedTranscodeConfig(t, handler) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("server", func(t *testing.T) { checkNamedServerConfig(t, handler) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("thirdLogin", func(t *testing.T) { checkNamedThirdLoginConfig(t, handler) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("ai", func(t *testing.T) { checkNamedAIConfig(t, handler) })
}

func TestAdminUpdateConfigItemViewPreservesMaskedThirdLoginSecret(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")

	initialConfig := []byte(`thirdlogin:
  google:
    clientID: "google-old"
    clientSecret: "google-secret-old"
    redirect: "https://example.com/oauth/google-old"
`)
	{
		err := os.WriteFile(configPath, initialConfig, 0o600)
		require.NoError(t, err, "write temp config failed: %v", err)
	}

	handler, provider := newConfigHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			Google: &thirdlogin.Google{
				ClientID:     "google-old",
				ClientSecret: "google-secret-old",
				Redirect:     "https://example.com/oauth/google-old",
			},
		},
	}, configPath)

	body := `{
		"google": {
			"clientID": "google-new",
			"clientSecret": "******",
			"redirect": "https://example.com/oauth/google-new"
		}
	}`

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/admin/site/config?name="+configNameThirdLogin, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.AdminUpdateConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	require.Equal(t, "google-secret-old", provider.Get().ThirdLogin.Google.ClientSecret, "masked secret should preserve old google clientSecret, got: %s", provider.Get().ThirdLogin.Google.ClientSecret)

	content, err := os.ReadFile(configPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err, "read temp config failed: %v", err)

	require.Contains(t, string(content), "google-secret-old", "updated config file should preserve old google clientSecret, got: %s", string(content))
}

func TestAdminConfigItemViewRejectsUnsupportedName(t *testing.T) {
	t.Parallel()

	handler, _ := newConfigHandler(t, &config.Config{}, filepath.Join(t.TempDir(), "config.yaml"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=unknown", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	require.EqualValues(t, res.FailServiceCode, response.Code, "unexpected response code: %d", response.Code)

	require.Equal(t, "不支持的配置项", response.Msg, "unexpected response msg: %s", response.Msg)
}

func TestAdminUpdateConfigItemViewUpdatesThirdLogin(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")

	initialConfig := []byte(`thirdlogin:
  qq:
    appid: "qq-old"
    appkey: "qq-secret"
    redirect: "https://example.com/oauth/qq"
  google:
    clientID: "google-old"
    clientSecret: "google-secret-old"
    redirect: "https://example.com/oauth/google-old"
`)
	{
		err := os.WriteFile(configPath, initialConfig, 0o600)
		require.NoError(t, err, "write temp config failed: %v", err)
	}

	handler, provider := newConfigHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			QQ: &thirdlogin.QQ{
				AppID:    "qq-old",
				AppKey:   "qq-secret",
				Redirect: "https://example.com/oauth/qq",
			},
			Google: &thirdlogin.Google{
				ClientID:     "google-old",
				ClientSecret: "google-secret-old",
				Redirect:     "https://example.com/oauth/google-old",
			},
		},
	}, configPath)

	body := `{
		"qq": {
			"appID": "qq-old",
			"appKey": "qq-secret",
			"redirect": "https://example.com/oauth/qq"
		},
		"google": {
			"clientID": "google-new",
			"clientSecret": "google-secret-new",
			"redirect": "https://example.com/oauth/google-new"
		}
	}`

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/admin/site/config?name="+configNameThirdLogin, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.AdminUpdateConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d", response.Code)

	require.NotNil(t, provider.Get().ThirdLogin, "global config should contain updated google settings")
	require.NotNil(t, provider.Get().ThirdLogin.Google, "global config should contain updated google settings")

	require.Equal(t, "google-new", provider.Get().ThirdLogin.Google.ClientID, "unexpected google clientID: %s", provider.Get().ThirdLogin.Google.ClientID)

	require.Equal(t, "https://example.com/oauth/google-new", provider.Get().ThirdLogin.Google.Redirect, "unexpected google redirect: %s", provider.Get().ThirdLogin.Google.Redirect)

	content, err := os.ReadFile(configPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err, "read temp config failed: %v", err)

	require.Contains(t, string(content), "google-new", "updated config file should contain new google clientID, got: %s", string(content))

	require.Contains(t, string(content), "google-secret-new", "updated config file should contain new google clientSecret, got: %s", string(content))
}

func TestAdminUpdateConfigItemViewUpdatesAI(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")

	initialConfig := []byte(`ai:
  chatmodelbaseurl: "https://api.deepseek.com/anthropic"
  chatmodelapikey: "old-deepseek-key"
  chatmodel: "deepseek-v4-flash"
  embeddingmodel: "doubao-embedding-vision-251215"
  embeddingdimensions: 2048
  doubaomodelapikey: "old-doubao-key"
  systemprompt: "old prompt"
  imagemodel: "doubao-seedream-5-0-lite-260128"
  videomodel: "doubao-seedance-1-5-pro-251215"
  vectorindex: "search_videos_vector"
  vectortopk: 12
  timeoutsec: 30
`)
	{
		err := os.WriteFile(configPath, initialConfig, 0o600)
		require.NoError(t, err, "write temp config failed: %v", err)
	}

	handler, provider := newConfigHandler(t, &config.Config{
		AI: &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
			ChatModelBaseURL:    "https://api.deepseek.com/anthropic",
			ChatModelAPIKey:     "old-deepseek-key",
			ChatModel:           testDeepseekV4Flash,
			EmbeddingModel:      "doubao-embedding-vision-251215",
			EmbeddingDimensions: 2048,
			DoubaoModelAPIKey:   "old-doubao-key",
			SystemPrompt:        "old prompt",
			ImageModel:          "doubao-seedream-5-0-lite-260128",
			VideoModel:          "doubao-seedance-1-5-pro-251215",
			VectorIndex:         "search_videos_vector",
			VectorTopK:          12,
			TimeoutSec:          30,
		},
	}, configPath)

	body := `{
		"chatModelBaseURL": "https://api.deepseek.com/anthropic",
		"chatModelAPIKey": "new-deepseek-key",
		"chatModel": "deepseek-v4-flash",
		"embeddingModel": "doubao-embedding-vision-251215",
		"embeddingDimensions": 2048,
		"doubaoModelAPIKey": "new-doubao-key",
		"systemPrompt": "new prompt",
		"imageModel": "doubao-seedream-4-5-251128",
		"videoModel": "doubao-seedance-1-0-pro-fast-251015",
		"vectorIndex": "search_videos_vector",
		"vectorTopK": 20,
		"timeoutSec": 60
	}`

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/admin/site/config?name=ai", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	handler.AdminUpdateConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d", response.Code)

	require.NotNil(t, provider.Get().AI, "global config should contain updated ai settings")

	require.Equal(t, "new-deepseek-key", provider.Get().AI.ChatModelAPIKey, "unexpected ai chatModelAPIKey: %s", provider.Get().AI.ChatModelAPIKey)

	require.Equal(t, "new-doubao-key", provider.Get().AI.DoubaoModelAPIKey, "unexpected ai doubaoModelAPIKey: %s", provider.Get().AI.DoubaoModelAPIKey)

	require.Equal(t, "new prompt", provider.Get().AI.SystemPrompt, "unexpected ai systemPrompt: %s", provider.Get().AI.SystemPrompt)

	require.Equal(t, "doubao-seedream-4-5-251128", provider.Get().AI.ImageModel, "unexpected ai imageModel: %s", provider.Get().AI.ImageModel)

	require.Equal(t, "doubao-seedance-1-0-pro-fast-251015", provider.Get().AI.VideoModel, "unexpected ai videoModel: %s", provider.Get().AI.VideoModel)

	content, err := os.ReadFile(configPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err, "read temp config failed: %v", err)

	require.Contains(t, string(content), "new-deepseek-key", "updated config file should contain new ai chatModelAPIKey, got: %s", string(content))

	require.Contains(t, string(content), "new-doubao-key", "updated config file should contain new ai doubaoModelAPIKey, got: %s", string(content))

	require.Contains(t, string(content), "new prompt", "updated config file should contain new ai systemPrompt, got: %s", string(content))
}

func TestCommonConfigViewHidesStorageConfig(t *testing.T) {
	t.Parallel()

	handler, provider := newConfigHandler(t, &config.Config{
		Site: &site.Site{
			DefaultUserBannerID:  19,
			DefaultUserBannerIDs: []uint{19, 23},
			Login: &site.Login{
				QQLogin:          true,
				UsernamePwdLogin: true,
			},
			Register: &site.Register{
				EmailCaptcha: true,
			},
			ContentReview: &site.ContentReview{
				Enable: true,
			},
			Storage: &site.Storage{
				ChunkDir: "runtime/chunks",
				Local: &site.Local{
					Enable: true,
					Path:   "uploads",
				},
			},
		},
	}, filepath.Join(t.TempDir(), "config.yaml"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/common/site/config", nil)

	handler.CommonConfigView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	require.NotNil(t, provider.Get().Site.Storage, "global storage config should not be nil")

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	payload, err := json.Marshal(response.Data)
	require.NoError(t, err, "marshal response data failed: %v", err)

	var resp SiteResp
	{
		err = json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal site response failed: %v", err)
	}

	require.NotNil(t, resp.Site.Storage, "common config response should keep public storage settings")

	require.Nil(t, resp.Site.Storage.Local, "common config response should hide private storage fields, got %+v", resp.Site.Storage)
	require.Nil(t, resp.Site.Storage.MinIO, "common config response should hide private storage fields, got %+v", resp.Site.Storage)
	require.Empty(t, resp.Site.Storage.ChunkDir, "common config response should hide private storage fields, got %+v", resp.Site.Storage)

	require.NotNil(t, resp.Site.Login, "common config should keep login settings")
	require.True(t, resp.Site.Login.QQLogin, "common config should keep login settings")

	require.NotNil(t, resp.Site.Register, "common config should keep register settings")
	require.True(t, resp.Site.Register.EmailCaptcha, "common config should keep register settings")

	require.Len(t, resp.Site.DefaultUserBannerIDs, 2, "common config should keep default user banner ids, got %+v", resp.Site.DefaultUserBannerIDs)
	require.EqualValues(t, 19, resp.Site.DefaultUserBannerIDs[0], "common config should keep default user banner ids, got %+v", resp.Site.DefaultUserBannerIDs)
	require.EqualValues(t, 23, resp.Site.DefaultUserBannerIDs[1], "common config should keep default user banner ids, got %+v", resp.Site.DefaultUserBannerIDs)
}

func decodeResponseData(t *testing.T, recorder *httptest.ResponseRecorder) []byte {
	t.Helper()

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	payload, err := json.Marshal(response.Data)
	require.NoError(t, err, "marshal response data failed: %v", err)

	return payload
}

func TestSiteSubmissionPolicyRoundTripAndLegacyUpdates(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	{
		err := os.WriteFile(path, []byte("site: {}\n"), 0o600)
		require.NoError(t, err)
	}

	handler, provider := newConfigHandler(t, &config.Config{Site: &site.Site{}}, path)

	for _, step := range []struct {
		body string
		want bool
	}{
		{`{"disableUserSubmission":true,"login":{"usernamePwdLogin":true}}`, true},
		{`{"login":{"usernamePwdLogin":true}}`, true},
		{`{"disableUserSubmission":false,"login":{"usernamePwdLogin":true}}`, false},
	} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/v1/admin/site/config?name=site", strings.NewReader(step.body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		handler.AdminUpdateConfigItemView(ctx)

		var response res.Response
		{
			err := json.Unmarshal(rec.Body.Bytes(), &response)
			require.False(t, err != nil || response.Code != 0, "update failed: %s", rec.Body.String())
		}

		require.Equal(t, step.want, provider.Get().Site.DisableUserSubmission, "wrong effective policy: %+v", provider.Get().Site)

		contents, err := os.ReadFile(path) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
		require.NoError(t, err)

		var persisted struct {
			Site site.Site `yaml:"site"`
		}
		{
			err := yaml.Unmarshal(contents, &persisted)
			require.NoError(t, err)
		}

		require.Equal(t, step.want, persisted.Site.DisableUserSubmission, "policy not persisted: %s", contents)

		for _, admin := range []bool{false, true} {
			rec = httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(rec)

			ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/admin/site/config?name=site", nil)
			if admin {
				handler.AdminConfigItemView(ctx)
			} else {
				handler.CommonConfigView(ctx)
			}

			var result struct {
				Data json.RawMessage `json:"data"`
			}
			{
				err := json.Unmarshal(rec.Body.Bytes(), &result)
				require.NoError(t, err)
			}

			var cfg site.Site

			if admin {
				{
					err := json.Unmarshal(result.Data, &cfg)
					require.NoError(t, err)
				}
			} else {
				var public SiteResp
				{
					err := json.Unmarshal(result.Data, &public)
					require.NoError(t, err)
				}

				cfg = public.Site
			}

			require.Equal(t, step.want, cfg.DisableUserSubmission, "policy not exposed to admin=%v: %s", admin, rec.Body.String())
		}
	}
}

func checkNamedSiteConfig(t *testing.T, handler *Handler, provider *infraconfig.Provider) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=site", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	require.NotNil(t, provider.Get().Site.Storage, "global storage config should not be nil")

	payload := decodeResponseData(t, recorder)

	var data map[string]any
	{
		err := json.Unmarshal(payload, &data)
		require.NoError(t, err, "unmarshal admin site response failed: %v", err)
	}

	{
		_, ok := data[configNameSite]
		require.False(t, ok, "admin site config response should not wrap payload in data.site")
	}

	var resp site.Site
	{
		err := json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal site config failed: %v", err)
	}

	require.NotNil(t, resp.Login, "admin site config should contain login settings")
	require.True(t, resp.Login.QQLogin, "admin site config should contain login settings")

	require.NotNil(t, resp.Storage, "admin site config should contain full storage settings")
	require.Equal(t, "runtime/chunks", resp.Storage.ChunkDir, "admin site config should contain full storage settings")

	require.NotNil(t, resp.Storage.Local, "admin site config should keep local storage settings")
	require.True(t, resp.Storage.Local.Enable, "admin site config should keep local storage settings")

	require.NotNil(t, resp.Storage.MinIO, "admin site config should keep minio settings")
	require.True(t, resp.Storage.MinIO.Enable, "admin site config should keep minio settings")

	require.Equal(t, "minio-secret-key", resp.Storage.MinIO.SecretKey, "admin site config should return plain minio secretKey, got: %s", resp.Storage.MinIO.SecretKey)
}

func checkNamedTranscodeConfig(t *testing.T, handler *Handler) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=transcode", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	payload := decodeResponseData(t, recorder)

	var data map[string]any
	{
		err := json.Unmarshal(payload, &data)
		require.NoError(t, err, "unmarshal admin transcode response failed: %v", err)
	}

	{
		_, ok := data[configNameSite]
		require.False(t, ok, "admin transcode config response should not return site config")
	}

	var resp config.Transcode
	{
		err := json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal transcode config failed: %v", err)
	}

	require.Equal(t, 4, resp.MaxWorkers, "unexpected maxWorkers: %d", resp.MaxWorkers)

	require.Len(t, resp.TranscodeResolutions, 3, "unexpected transcode resolutions length: %d", len(resp.TranscodeResolutions))

	require.True(t, resp.GpuMode, "admin transcode config should contain gpuMode")
}

func checkNamedServerConfig(t *testing.T, handler *Handler) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=server", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	payload := decodeResponseData(t, recorder)

	var resp AdminServerCORSConfig
	{
		err := json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal server cors config failed: %v", err)
	}

	require.Len(t, resp.CorsOrigins, 2, "unexpected corsOrigins length: %d", len(resp.CorsOrigins))

	require.Equal(t, "http://localhost:3000", resp.CorsOrigins[0], "unexpected first origin: %s", resp.CorsOrigins[0])

	// 管理端响应不应泄露完整 server 监听字段（仅 corsOrigins）。
	var raw map[string]any
	{
		err := json.Unmarshal(payload, &raw)
		require.NoError(t, err, "unmarshal raw failed: %v", err)
	}

	{
		_, ok := raw["addr"]
		require.False(t, ok, "admin server config should not expose addr")
	}
}

func checkNamedThirdLoginConfig(t *testing.T, handler *Handler) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=thirdLogin", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	payload := decodeResponseData(t, recorder)

	var resp config.ThirdLogin
	{
		err := json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal third login config failed: %v", err)
	}

	require.NotNil(t, resp.Google, "admin third login config should contain google settings")

	require.Equal(t, testGoogleClientID, resp.Google.ClientID, "unexpected google clientID: %s", resp.Google.ClientID)

	require.Equal(t, testGoogleRedirect, resp.Google.Redirect, "unexpected google redirect: %s", resp.Google.Redirect)

	require.NotNil(t, resp.QQ, "admin third login config should contain qq settings")
	require.Equal(t, "qq-app-id", resp.QQ.AppID, "admin third login config should contain qq settings")

	require.Equal(t, "qq-app-key", resp.QQ.AppKey, "admin third login config should return plain qq appKey, got: %s", resp.QQ.AppKey)

	require.Equal(t, "google-client-secret", resp.Google.ClientSecret, "admin third login config should return plain google clientSecret, got: %s", resp.Google.ClientSecret)
}

func checkNamedAIConfig(t *testing.T, handler *Handler) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=ai", nil)

	handler.AdminConfigItemView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	payload := decodeResponseData(t, recorder)

	var resp config.AI
	{
		err := json.Unmarshal(payload, &resp)
		require.NoError(t, err, "unmarshal ai config failed: %v", err)
	}

	require.Equal(t, "https://api.deepseek.com/anthropic", resp.ChatModelBaseURL, "unexpected ai chatModelBaseURL: %s", resp.ChatModelBaseURL)

	require.Equal(t, "deepseek-test-key", resp.ChatModelAPIKey, "admin ai config should return plain chatModelAPIKey, got: %s", resp.ChatModelAPIKey)

	require.Equal(t, "doubao-test-key", resp.DoubaoModelAPIKey, "admin ai config should return plain doubaoModelAPIKey, got: %s", resp.DoubaoModelAPIKey)

	require.Equal(t, testDeepseekV4Flash, resp.ChatModel, "unexpected ai chatModel: %s", resp.ChatModel)

	require.True(t, resp.ThinkingEnabled, "admin ai config should enable thinking by default in fixture")

	require.Equal(t, "high", resp.ThinkingEffort, "unexpected ai thinkingEffort: %s", resp.ThinkingEffort)

	require.Len(t, resp.ChatModels, 2, "unexpected ai chatModels len: %d", len(resp.ChatModels))

	require.Len(t, resp.ThinkingEfforts, 2, "unexpected ai thinkingEfforts len: %d", len(resp.ThinkingEfforts))

	require.Equal(t, "doubao-embedding-vision-251215", resp.EmbeddingModel, "unexpected ai embeddingModel: %s", resp.EmbeddingModel)

	require.NotEmpty(t, resp.SystemPrompt, "admin ai config should contain systemPrompt")
}

const testDeepseekV4Flash = "deepseek-v4-flash"
