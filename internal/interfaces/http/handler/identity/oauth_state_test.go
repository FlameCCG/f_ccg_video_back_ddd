package identity

import (
	"net/http"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/external/oauth"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// newOAuthLoginHandler 构造只装配第三方登录用例的处理器。
//
// 原测试把 global.RedisClient 置空、让 login_service 回落到内存 map 来制造「无效 state」；
// 这里注入 nil Redis 的 StateStore，语义完全相同（内存兜底且未写入过 state）。
func newOAuthLoginHandler(t *testing.T, login *site.Login) *Handler {
	t.Helper()

	provider := config.NewProvider(&config.Config{Site: &site.Site{Login: login}})
	states := oauth.NewStateStore(nil)

	oauthLogins := app.NewOAuthLoginService(
		provider, nil, nil, nil, nil, states, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop(),
	)

	return NewHandler(nil, nil, nil, nil, nil, nil, nil, oauthLogins, nil, nil, nil, nil, nil, zap.NewNop())
}

func TestGitHubLoginViewRejectsInvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newOAuthLoginHandler(t, &site.Login{GitHubLogin: true})

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/user/login/github")
	ctx.Set("request", GitHubLoginReq{
		Code:  "github-code",
		State: "invalid-state",
	})

	handler.GitHubLoginView(ctx)

	response := decodeResponse(t, recorder)
	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "github登录状态无效" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

func TestXLoginViewRejectsInvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newOAuthLoginHandler(t, &site.Login{XLogin: true})

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/user/login/x")
	ctx.Set("request", XLoginReq{
		Code:         "x-code",
		State:        "invalid-state",
		CodeVerifier: "verifier",
	})

	handler.XLoginView(ctx)

	response := decodeResponse(t, recorder)
	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "x登录状态无效" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

func TestLinuxDoLoginViewRejectsInvalidState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newOAuthLoginHandler(t, &site.Login{LinuxDoLogin: true})

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/user/login/linuxdo")
	ctx.Set("request", LinuxDoLoginReq{
		Code:  "linuxdo-code",
		State: "invalid-state",
	})

	handler.LinuxDoLoginView(ctx)

	response := decodeResponse(t, recorder)
	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "linuxdo登录状态无效" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}
