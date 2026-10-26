package contentops

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

// fakeOAuthStateStore 记录被保存的一次性 state，替代 identity 上下文的真实实现。
//
// 原测试通过把 global.RedisClient 置空让 login_service 回落到内存 map，
// 再调用 ConsumeOAuthState 断言 state 已落库；这里用注入的 fake 达到同样的断言强度。
type fakeOAuthStateStore struct {
	mu     sync.Mutex
	stored map[string]struct{}
	err    error
}

func newFakeOAuthStateStore() *fakeOAuthStateStore {
	return &fakeOAuthStateStore{stored: make(map[string]struct{})}
}

// StoreOAuthState 保存 state。
func (s *fakeOAuthStateStore) StoreOAuthState(_ context.Context, provider, state string) error {
	if s.err != nil {
		return s.err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stored[provider+":"+state] = struct{}{}

	return nil
}

// consume 校验并消费 state，语义与原 login_service.ConsumeOAuthState 一致。
func (s *fakeOAuthStateStore) consume(provider, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := provider + ":" + state
	if _, ok := s.stored[key]; !ok {
		return errors.New("oauth state is invalid")
	}

	delete(s.stored, key)

	return nil
}

// newOAuthHandler 构造只装配三方登录授权链接用例的处理器。
func newOAuthHandler(t *testing.T, cfg *config.Config) (*Handler, *fakeOAuthStateStore) {
	t.Helper()

	states := newFakeOAuthStateStore()
	provider := infraconfig.NewProvider(cfg)

	return NewHandler(nil, nil, nil, app.NewOAuthURLService(provider, states), nil), states
}

func TestGitHubUrlViewStoresState(t *testing.T) {
	t.Parallel()

	handler, states := oauthURLTestProvider(t, "github")
	recorder := performOAuthURLRequest(t, handler.GitHubURLView, "github", GitHubURLReq{State: "github-state"})

	assertStoredOAuthState(t, recorder, states, "github", "github-state")
}

func TestXUrlViewStoresState(t *testing.T) {
	t.Parallel()

	handler, states := newOAuthHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			X: &thirdlogin.X{
				ClientID:     "x-client-id",
				ClientSecret: "x-secret",
				Redirect:     "http://localhost:3000/oauth/x",
			},
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/common/site/x-url", nil)
	ctx.Set("request", XURLReq{
		State:         "x-state",
		CodeChallenge: "challenge",
	})

	handler.XUrlView(ctx)

	assertStoredOAuthState(t, recorder, states, "x", "x-state")
}

func TestLinuxDoUrlViewStoresState(t *testing.T) {
	t.Parallel()

	handler, states := oauthURLTestProvider(t, "linuxdo")
	recorder := performOAuthURLRequest(t, handler.LinuxDoURLView, "linuxdo", LinuxDoURLReq{State: "linuxdo-state"})

	assertStoredOAuthState(t, recorder, states, "linuxdo", "linuxdo-state")
}

func TestQQURLStoresOneTimeServerState(t *testing.T) {
	t.Parallel()

	handler, states := newOAuthHandler(t, &config.Config{ThirdLogin: &config.ThirdLogin{QQ: &thirdlogin.QQ{AppID: "app", AppKey: "secret", Redirect: "https://example.com/qq"}}})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/site/qq-url", nil)
	handler.QQUrlView(ctx)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err)
	}

	raw, ok := response.Data.(string)
	require.True(t, ok, "missing authorization URL")

	parsed, err := url.Parse(raw)
	require.NoError(t, err)

	state := parsed.Query().Get("state")
	require.NotEmpty(t, state, "QQ state missing")

	{
		err := states.consume("qq", state)
		require.NoError(t, err)
	}

	{
		err := states.consume("qq", state)
		require.Error(t, err, "QQ state reused")
	}
}

func TestOAuthMissingConfigDoesNotStoreState(t *testing.T) {
	t.Parallel()

	states := newFakeOAuthStateStore()
	provider := infraconfig.NewProvider(&config.Config{})
	service := app.NewOAuthURLService(provider, states)

	calls := []func() (string, error){
		func() (string, error) { return service.GoogleURL(t.Context(), "caller-state") },
		func() (string, error) {
			return service.GitHubURL(t.Context(), app.GitHubURLParams{State: "caller-state"})
		},
		func() (string, error) { return service.LinuxDoURL(t.Context(), "caller-state") },
		func() (string, error) { return service.XURL(t.Context(), app.XURLParams{State: "caller-state"}) },
		func() (string, error) { return service.QQURL(t.Context()) },
	}
	for _, call := range calls {
		{
			raw, err := call()
			require.NoError(t, err, "raw=%q err=%v", raw, err)
			require.Empty(t, raw, "raw=%q err=%v", raw, err)
		}
	}

	require.Empty(t, states.stored, "missing configuration polluted state store: %v", states.stored)
}

func assertStoredOAuthState(t *testing.T, recorder *httptest.ResponseRecorder, states *fakeOAuthStateStore, provider, rejectedState string) {
	t.Helper()

	var response res.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.EqualValues(t, res.SuccessCode, response.Code)
	authURL, ok := response.Data.(string)
	require.True(t, ok, "authorization URL is %T", response.Data)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)
	require.NotEqual(t, rejectedState, state, "state must be server generated")
	require.NoError(t, states.consume(provider, state))
}

func oauthURLTestProvider(t *testing.T, provider string) (*Handler, *fakeOAuthStateStore) {
	t.Helper()

	login := &config.ThirdLogin{}

	switch provider {
	case "github":
		login.GitHub = &thirdlogin.GitHub{ClientID: "github-client-id", ClientSecret: "github-secret", Redirect: "http://localhost:3000/oauth/github"}
	case "linuxdo":
		login.LinuxDo = &thirdlogin.LinuxDo{ClientID: "linuxdo-client-id", ClientSecret: "linuxdo-secret", Redirect: "http://localhost:3000/oauth/linuxdo"}
	default:
		t.Fatalf("unsupported test provider %q", provider)
	}

	return newOAuthHandler(t, &config.Config{ThirdLogin: login})
}

func performOAuthURLRequest(t *testing.T, serve func(*gin.Context), provider string, request any) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/common/site/"+provider+"-url", nil)
	c.Set("request", request)
	serve(c)

	return recorder
}
