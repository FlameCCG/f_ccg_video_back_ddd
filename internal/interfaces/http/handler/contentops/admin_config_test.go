package contentops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
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
	gin.SetMode(gin.TestMode)

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
		AI: &config.AI{
			ChatModelBaseURL: "https://api.deepseek.com/anthropic",
			ChatModelAPIKey:  "deepseek-test-key",
			ChatModel:        "deepseek-v4-flash",
			ChatModels: []config.AIModel{
				{Label: "DeepSeek V4 Flash", Value: "deepseek-v4-flash"},
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

	t.Run("site", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=site", nil)

		handler.AdminConfigItemView(ctx)

		if recorder.Code != 200 {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		if provider.Get().Site.Storage == nil {
			t.Fatal("global storage config should not be nil")
		}

		payload := decodeResponseData(t, recorder)

		var data map[string]any
		if err := json.Unmarshal(payload, &data); err != nil {
			t.Fatalf("unmarshal admin site response failed: %v", err)
		}

		if _, ok := data[configNameSite]; ok {
			t.Fatal("admin site config response should not wrap payload in data.site")
		}

		var resp site.Site
		if err := json.Unmarshal(payload, &resp); err != nil {
			t.Fatalf("unmarshal site config failed: %v", err)
		}

		if resp.Login == nil || !resp.Login.QQLogin {
			t.Fatal("admin site config should contain login settings")
		}

		if resp.Storage == nil || resp.Storage.ChunkDir != "runtime/chunks" {
			t.Fatal("admin site config should contain full storage settings")
		}

		if resp.Storage.Local == nil || !resp.Storage.Local.Enable {
			t.Fatal("admin site config should keep local storage settings")
		}

		if resp.Storage.MinIO == nil || !resp.Storage.MinIO.Enable {
			t.Fatal("admin site config should keep minio settings")
		}

		if resp.Storage.MinIO.SecretKey != "minio-secret-key" {
			t.Fatalf("admin site config should return plain minio secretKey, got: %s", resp.Storage.MinIO.SecretKey)
		}
	})

	t.Run("transcode", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=transcode", nil)

		handler.AdminConfigItemView(ctx)

		if recorder.Code != 200 {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		payload := decodeResponseData(t, recorder)

		var data map[string]any
		if err := json.Unmarshal(payload, &data); err != nil {
			t.Fatalf("unmarshal admin transcode response failed: %v", err)
		}

		if _, ok := data[configNameSite]; ok {
			t.Fatal("admin transcode config response should not return site config")
		}

		var resp config.Transcode
		if err := json.Unmarshal(payload, &resp); err != nil {
			t.Fatalf("unmarshal transcode config failed: %v", err)
		}

		if resp.MaxWorkers != 4 {
			t.Fatalf("unexpected maxWorkers: %d", resp.MaxWorkers)
		}

		if len(resp.TranscodeResolutions) != 3 {
			t.Fatalf("unexpected transcode resolutions length: %d", len(resp.TranscodeResolutions))
		}

		if !resp.GpuMode {
			t.Fatal("admin transcode config should contain gpuMode")
		}
	})

	t.Run("server", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=server", nil)

		handler.AdminConfigItemView(ctx)

		if recorder.Code != 200 {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		payload := decodeResponseData(t, recorder)

		var resp AdminServerCORSConfig
		if err := json.Unmarshal(payload, &resp); err != nil {
			t.Fatalf("unmarshal server cors config failed: %v", err)
		}

		if len(resp.CorsOrigins) != 2 {
			t.Fatalf("unexpected corsOrigins length: %d", len(resp.CorsOrigins))
		}

		if resp.CorsOrigins[0] != "http://localhost:3000" {
			t.Fatalf("unexpected first origin: %s", resp.CorsOrigins[0])
		}

		// 管理端响应不应泄露完整 server 监听字段（仅 corsOrigins）。
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			t.Fatalf("unmarshal raw failed: %v", err)
		}

		if _, ok := raw["addr"]; ok {
			t.Fatal("admin server config should not expose addr")
		}
	})

	t.Run("thirdLogin", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=thirdLogin", nil)

		handler.AdminConfigItemView(ctx)

		if recorder.Code != 200 {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		payload := decodeResponseData(t, recorder)

		var resp config.ThirdLogin
		if err := json.Unmarshal(payload, &resp); err != nil {
			t.Fatalf("unmarshal third login config failed: %v", err)
		}

		if resp.Google == nil {
			t.Fatal("admin third login config should contain google settings")
		}

		if resp.Google.ClientID != testGoogleClientID {
			t.Fatalf("unexpected google clientID: %s", resp.Google.ClientID)
		}

		if resp.Google.Redirect != testGoogleRedirect {
			t.Fatalf("unexpected google redirect: %s", resp.Google.Redirect)
		}

		if resp.QQ == nil || resp.QQ.AppID != "qq-app-id" {
			t.Fatal("admin third login config should contain qq settings")
		}

		if resp.QQ.AppKey != "qq-app-key" {
			t.Fatalf("admin third login config should return plain qq appKey, got: %s", resp.QQ.AppKey)
		}

		if resp.Google.ClientSecret != "google-client-secret" {
			t.Fatalf("admin third login config should return plain google clientSecret, got: %s", resp.Google.ClientSecret)
		}
	})

	t.Run("ai", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=ai", nil)

		handler.AdminConfigItemView(ctx)

		if recorder.Code != 200 {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		payload := decodeResponseData(t, recorder)

		var resp config.AI
		if err := json.Unmarshal(payload, &resp); err != nil {
			t.Fatalf("unmarshal ai config failed: %v", err)
		}

		if resp.ChatModelBaseURL != "https://api.deepseek.com/anthropic" {
			t.Fatalf("unexpected ai chatModelBaseURL: %s", resp.ChatModelBaseURL)
		}

		if resp.ChatModelAPIKey != "deepseek-test-key" {
			t.Fatalf("admin ai config should return plain chatModelAPIKey, got: %s", resp.ChatModelAPIKey)
		}

		if resp.DoubaoModelAPIKey != "doubao-test-key" {
			t.Fatalf("admin ai config should return plain doubaoModelAPIKey, got: %s", resp.DoubaoModelAPIKey)
		}

		if resp.ChatModel != "deepseek-v4-flash" {
			t.Fatalf("unexpected ai chatModel: %s", resp.ChatModel)
		}

		if !resp.ThinkingEnabled {
			t.Fatal("admin ai config should enable thinking by default in fixture")
		}

		if resp.ThinkingEffort != "high" {
			t.Fatalf("unexpected ai thinkingEffort: %s", resp.ThinkingEffort)
		}

		if len(resp.ChatModels) != 2 {
			t.Fatalf("unexpected ai chatModels len: %d", len(resp.ChatModels))
		}

		if len(resp.ThinkingEfforts) != 2 {
			t.Fatalf("unexpected ai thinkingEfforts len: %d", len(resp.ThinkingEfforts))
		}

		if resp.EmbeddingModel != "doubao-embedding-vision-251215" {
			t.Fatalf("unexpected ai embeddingModel: %s", resp.EmbeddingModel)
		}

		if resp.SystemPrompt == "" {
			t.Fatal("admin ai config should contain systemPrompt")
		}
	})
}

func TestAdminUpdateConfigItemViewPreservesMaskedThirdLoginSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configPath := filepath.Join(t.TempDir(), "config.yaml")

	initialConfig := []byte(`thirdlogin:
  google:
    clientID: "google-old"
    clientSecret: "google-secret-old"
    redirect: "https://example.com/oauth/google-old"
`)
	if err := os.WriteFile(configPath, initialConfig, 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
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

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	if provider.Get().ThirdLogin.Google.ClientSecret != "google-secret-old" {
		t.Fatalf("masked secret should preserve old google clientSecret, got: %s", provider.Get().ThirdLogin.Google.ClientSecret)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read temp config failed: %v", err)
	}

	if !strings.Contains(string(content), "google-secret-old") {
		t.Fatalf("updated config file should preserve old google clientSecret, got: %s", string(content))
	}
}

func TestAdminConfigItemViewRejectsUnsupportedName(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _ := newConfigHandler(t, &config.Config{}, filepath.Join(t.TempDir(), "config.yaml"))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/admin/site/config?name=unknown", nil)

	handler.AdminConfigItemView(ctx)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.FailServiceCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if response.Msg != "不支持的配置项" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

func TestAdminUpdateConfigItemViewUpdatesThirdLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	if err := os.WriteFile(configPath, initialConfig, 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
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

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if provider.Get().ThirdLogin == nil || provider.Get().ThirdLogin.Google == nil {
		t.Fatal("global config should contain updated google settings")
	}

	if provider.Get().ThirdLogin.Google.ClientID != "google-new" {
		t.Fatalf("unexpected google clientID: %s", provider.Get().ThirdLogin.Google.ClientID)
	}

	if provider.Get().ThirdLogin.Google.Redirect != "https://example.com/oauth/google-new" {
		t.Fatalf("unexpected google redirect: %s", provider.Get().ThirdLogin.Google.Redirect)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read temp config failed: %v", err)
	}

	if !strings.Contains(string(content), "google-new") {
		t.Fatalf("updated config file should contain new google clientID, got: %s", string(content))
	}

	if !strings.Contains(string(content), "google-secret-new") {
		t.Fatalf("updated config file should contain new google clientSecret, got: %s", string(content))
	}
}

func TestAdminUpdateConfigItemViewUpdatesAI(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	if err := os.WriteFile(configPath, initialConfig, 0o644); err != nil {
		t.Fatalf("write temp config failed: %v", err)
	}

	handler, provider := newConfigHandler(t, &config.Config{
		AI: &config.AI{
			ChatModelBaseURL:    "https://api.deepseek.com/anthropic",
			ChatModelAPIKey:     "old-deepseek-key",
			ChatModel:           "deepseek-v4-flash",
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

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if provider.Get().AI == nil {
		t.Fatal("global config should contain updated ai settings")
	}

	if provider.Get().AI.ChatModelAPIKey != "new-deepseek-key" {
		t.Fatalf("unexpected ai chatModelAPIKey: %s", provider.Get().AI.ChatModelAPIKey)
	}

	if provider.Get().AI.DoubaoModelAPIKey != "new-doubao-key" {
		t.Fatalf("unexpected ai doubaoModelAPIKey: %s", provider.Get().AI.DoubaoModelAPIKey)
	}

	if provider.Get().AI.SystemPrompt != "new prompt" {
		t.Fatalf("unexpected ai systemPrompt: %s", provider.Get().AI.SystemPrompt)
	}

	if provider.Get().AI.ImageModel != "doubao-seedream-4-5-251128" {
		t.Fatalf("unexpected ai imageModel: %s", provider.Get().AI.ImageModel)
	}

	if provider.Get().AI.VideoModel != "doubao-seedance-1-0-pro-fast-251015" {
		t.Fatalf("unexpected ai videoModel: %s", provider.Get().AI.VideoModel)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read temp config failed: %v", err)
	}

	if !strings.Contains(string(content), "new-deepseek-key") {
		t.Fatalf("updated config file should contain new ai chatModelAPIKey, got: %s", string(content))
	}

	if !strings.Contains(string(content), "new-doubao-key") {
		t.Fatalf("updated config file should contain new ai doubaoModelAPIKey, got: %s", string(content))
	}

	if !strings.Contains(string(content), "new prompt") {
		t.Fatalf("updated config file should contain new ai systemPrompt, got: %s", string(content))
	}
}

func TestCommonConfigViewHidesStorageConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	if provider.Get().Site.Storage == nil {
		t.Fatal("global storage config should not be nil")
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	payload, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal response data failed: %v", err)
	}

	var resp SiteResp
	if err = json.Unmarshal(payload, &resp); err != nil {
		t.Fatalf("unmarshal site response failed: %v", err)
	}

	if resp.Site.Storage == nil {
		t.Fatal("common config response should keep public storage settings")
	}

	if resp.Site.Storage.Local != nil || resp.Site.Storage.MinIO != nil || resp.Site.Storage.ChunkDir != "" {
		t.Fatalf("common config response should hide private storage fields, got %+v", resp.Site.Storage)
	}

	if resp.Site.Login == nil || !resp.Site.Login.QQLogin {
		t.Fatal("common config should keep login settings")
	}

	if resp.Site.Register == nil || !resp.Site.Register.EmailCaptcha {
		t.Fatal("common config should keep register settings")
	}

	if len(resp.Site.DefaultUserBannerIDs) != 2 || resp.Site.DefaultUserBannerIDs[0] != 19 || resp.Site.DefaultUserBannerIDs[1] != 23 {
		t.Fatalf("common config should keep default user banner ids, got %+v", resp.Site.DefaultUserBannerIDs)
	}
}

func decodeResponseData(t *testing.T, recorder *httptest.ResponseRecorder) []byte {
	t.Helper()

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	payload, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal response data failed: %v", err)
	}

	return payload
}
