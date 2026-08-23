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
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestRegisterEmailRouteDoesNotRequireSlideCaptcha 守住「注册邮箱路由不挂滑块验证码中间件」。
//
// 原 router/user_router_test.go 通过 global.SetConfig 打开滑块验证码开关；
// 这里改为注入 *infraconfig.Provider——开关同样为开，若注册路由误挂
// SlideCaptchaVerify，请求就会在中间件阶段被拒（而不是走到 JSON 绑定）。
func TestRegisterEmailRouteDoesNotRequireSlideCaptcha(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := infraconfig.NewProvider(&config.Config{
		Site: &site.Site{
			Register: &site.Register{
				EmailCaptcha: false,
				SlideCaptcha: true,
			},
		},
	})

	// 只装配验证码用例：注册路由链路只会用到验证码开关。
	captchaSvc := identityapp.NewCaptchaService(nil, nil, nil, nil, provider)
	mw := middlewares.New(provider, nil, nil, nil, nil, captchaSvc, nil, zap.NewNop())

	engine := gin.New()
	engine.Use(mw.InjectLogger())
	engine.Use(validate.New().Middleware())
	commonGroup := engine.Group("/v1/common")
	registerIdentityCommonRoutes(commonGroup, mw, nil)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/common/user/register/email",
		strings.NewReader(`{}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Code != res.FailServiceCode {
		t.Fatalf("expected business failure code %d, got %d", res.FailServiceCode, response.Code)
	}

	if response.Msg == "slide captcha verification required" {
		t.Fatalf("register route should not require slide captcha, got %q", response.Msg)
	}

	if response.Msg == captcha.ErrCaptchaExpired.Error() {
		t.Fatalf("register route should not verify slide captcha token again, got %q", response.Msg)
	}

	if !strings.Contains(response.Msg, "用户名") {
		t.Fatalf("expected request to reach JSON binding stage, got %q", response.Msg)
	}
}

// TestAdminLoginRouteRequiresSlideCaptcha 反向守卫：管理端登录路由必须挂滑块验证码中间件
// （原 router/user_admin_router.go 的中间件顺序）。
func TestAdminLoginRouteRequiresSlideCaptcha(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := infraconfig.NewProvider(&config.Config{
		Site: &site.Site{
			Register: &site.Register{SlideCaptcha: true},
		},
	})

	captchaSvc := identityapp.NewCaptchaService(nil, nil, nil, nil, provider)
	mw := middlewares.New(provider, nil, nil, nil, nil, captchaSvc, nil, zap.NewNop())

	engine := gin.New()
	engine.Use(mw.InjectLogger())
	adminGroup := engine.Group("/v1/admin")
	registerIdentityAdminRoutes(adminGroup, mw, nil)

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/v1/admin/user/login/pwd",
		strings.NewReader(`{}`),
	)
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if response.Msg != "slide captcha verification required" {
		t.Fatalf("admin login route should require slide captcha, got %q", response.Msg)
	}
}
