package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestPasswordLoginGraphicCaptcha(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name            string
		loginEnabled    bool
		registerEnabled bool
		payload         string
		wantMessage     string
	}{
		{"login enabled requires captcha", true, false, `{"username":"tester"}`, "请填写图形验证码"},
		{"register switch does not enable login captcha", false, true, `{"username":"tester"}`, "密码"},
		{"both disabled", false, false, `{"username":"tester"}`, "密码"},
		{"wrong answer", true, false, `{"username":"tester","captchaID":"challenge","captchaCode":"wrong"}`, "图形验证码错误"},
		{"expired challenge", true, false, `{"username":"tester","captchaID":"expired","captchaCode":"42"}`, "图形验证码错误"},
		{"blank answer", true, false, `{"username":"tester","captchaID":"challenge","captchaCode":"  "}`, "请填写图形验证码"},
		{"correct answer preserves login body", true, false, `{"username":"tester","captchaID":"challenge","captchaCode":"42"}`, "密码"},
		{"both enabled accepts correct answer", true, true, `{"username":"tester","captchaID":"challenge","captchaCode":"42"}`, "密码"},
		{"disabled ignores stale answer", false, true, `{"username":"tester","captchaID":"expired","captchaCode":"wrong"}`, "密码"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{
				Login:    &site.Login{TextGraphicCaptcha: tt.loginEnabled},
				Register: &site.Register{TextGraphicCaptcha: tt.registerEnabled},
			}})
			store := captcha.NewGraphicStore()
			if err := store.Set("challenge", "42"); err != nil {
				t.Fatal(err)
			}
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			service := identityapp.NewCaptchaService(nil, nil, store, nil, provider)
			mw := middlewares.New(provider, nil, nil, nil, nil, nil, service, ratelimit.New(client), zap.NewNop())
			engine := gin.New()
			engine.Use(mw.InjectLogger(), validate.New().Middleware())
			registerIdentityCommonRoutes(engine.Group("/v1/common"), mw, nil)
			registerIdentityAdminRoutes(engine.Group("/v1/admin"), mw, nil)
			path := "/v1/common/user/login/pwd"

			post := func(want string) {
				t.Helper()
				req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
					path, strings.NewReader(tt.payload))
				req.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, req)
				var response res.Response
				if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Code != res.FailServiceCode || !strings.Contains(response.Msg, want) {
					t.Fatalf("want failure containing %q, got %+v", want, response)
				}
				if want == "密码" && strings.Contains(response.Msg, "用户名") {
					t.Fatalf("username was lost while reading captcha payload: %q", response.Msg)
				}
			}
			path = "/v1/admin/user/login/pwd"
			post(tt.wantMessage)
			if tt.loginEnabled && tt.wantMessage == "密码" {
				post("图形验证码错误") // 验证成功后不能重复使用。
			}
			if err := store.Set("challenge", "42"); err != nil {
				t.Fatal(err)
			}
			path = "/v1/common/user/login/pwd"
			post(tt.wantMessage)
			provider.Set(&config.Config{Site: &site.Site{
				Login: &site.Login{TextGraphicCaptcha: !tt.loginEnabled},
			}})
			if tt.loginEnabled {
				post("密码")
			} else {
				post("图形验证码")
			}
		})
	}
}

func TestEmailCaptchaGraphicSwitchIsIndependentOfLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, registerEnabled := range []bool{false, true} {
		name := "register disabled, login enabled"
		if registerEnabled {
			name = "register enabled, login disabled"
		}
		t.Run(name, func(t *testing.T) {
			provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{
				Login:    &site.Login{TextGraphicCaptcha: !registerEnabled},
				Register: &site.Register{TextGraphicCaptcha: registerEnabled},
			}})
			service := identityapp.NewCaptchaService(nil, nil, captcha.NewGraphicStore(), nil, provider)
			mw := middlewares.New(provider, nil, nil, nil, nil, nil, service, nil, zap.NewNop())
			engine := gin.New()
			engine.Use(mw.InjectLogger(), validate.New().Middleware())
			registerCaptchaRoutes(engine.Group("/v1/common"), mw, nil)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost,
				"/v1/common/captcha/email", strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			var response res.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			want := "必填"
			if registerEnabled {
				want = "请填写图形验证码"
			}
			if response.Code != res.FailServiceCode || !strings.Contains(response.Msg, want) {
				t.Fatalf("want failure containing %q, got %+v", want, response)
			}
		})
	}
}
