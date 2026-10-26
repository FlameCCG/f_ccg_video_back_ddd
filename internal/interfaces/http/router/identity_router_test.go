package router

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/validate"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TestRegisterEmailRouteDoesNotRequireSlideCaptcha 守住「注册邮箱路由不挂滑块验证码中间件」。
//
// 原 router/user_router_test.go 通过 global.SetConfig 打开滑块验证码开关；
// 这里改为注入 *infraconfig.Provider——开关同样为开，若注册路由误挂
// SlideCaptchaVerify，请求就会在中间件阶段被拒（而不是走到 JSON 绑定）。
//
//nolint:paralleltest // Router construction configures Gin process mode.
func TestRegisterEmailRouteDoesNotRequireSlideCaptcha(t *testing.T) {
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
	mw := middlewares.New(provider, nil, nil, nil, nil, nil, captchaSvc, nil, zap.NewNop())

	engine := gin.New()
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

	require.Equal(t, http.StatusOK, recorder.Code, "expected status 200, got %d", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response: %v", err)
	}

	require.EqualValues(t, res.FailServiceCode, response.Code, "expected business failure code %d, got %d", res.FailServiceCode, response.Code)

	require.NotEqual(t, "slide captcha verification required", response.Msg, "register route should not require slide captcha, got %q", response.Msg)

	require.NotEqual(t, response.Msg, captcha.ErrCaptchaExpired.Error(), "register route should not verify slide captcha token again, got %q", response.Msg)

	require.Contains(t, response.Msg, "用户名", "expected request to reach JSON binding stage, got %q", response.Msg)
}

// TestAdminLoginRouteRequiresSlideCaptcha 反向守卫：管理端登录路由必须挂滑块验证码中间件
// （原 router/user_admin_router.go 的中间件顺序）。
//
//nolint:paralleltest // Router construction configures Gin process mode.
func TestAdminLoginRouteRequiresSlideCaptcha(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{
		Site: &site.Site{
			Login: &site.Login{SlideCaptcha: true},
		},
	})

	captchaSvc := identityapp.NewCaptchaService(nil, nil, nil, nil, provider)
	mw := middlewares.New(provider, nil, nil, nil, nil, nil, captchaSvc, nil, zap.NewNop())

	engine := gin.New()
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
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response: %v", err)
	}

	require.Equal(t, "captcha verification required", response.Msg, "admin login route should require slide captcha, got %q", response.Msg)
}
