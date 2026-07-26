package contentops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	thirdlogin "f_ccg_video_back_ddd/internal/infrastructure/config/thirdlogin"
	"f_ccg_video_back_ddd/pkg/res"

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
	provider := config.NewProvider(cfg)

	return NewHandler(nil, nil, nil, app.NewOAuthURLService(provider, states), nil), states
}

func TestGitHubUrlViewStoresState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, states := newOAuthHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			GitHub: &thirdlogin.GitHub{
				ClientID:     "github-client-id",
				ClientSecret: "github-secret",
				Redirect:     "http://localhost:3000/oauth/github",
			},
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/common/site/github-url", nil)
	ctx.Set("request", GitHubURLReq{
		State: "github-state",
	})

	handler.GitHubUrlView(ctx)

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if err := states.consume("github", "github-state"); err != nil {
		t.Fatalf("expected github state stored, got err: %v", err)
	}
}

func TestXUrlViewStoresState(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if err := states.consume("x", "x-state"); err != nil {
		t.Fatalf("expected x state stored, got err: %v", err)
	}
}

func TestLinuxDoUrlViewStoresState(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, states := newOAuthHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			LinuxDo: &thirdlogin.LinuxDo{
				ClientID:     "linuxdo-client-id",
				ClientSecret: "linuxdo-secret",
				Redirect:     "http://localhost:3000/oauth/linuxdo",
			},
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/common/site/linuxdo-url", nil)
	ctx.Set("request", LinuxDoURLReq{
		State: "linuxdo-state",
	})

	handler.LinuxDoUrlView(ctx)

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d", response.Code)
	}

	if err := states.consume("linuxdo", "linuxdo-state"); err != nil {
		t.Fatalf("expected linuxdo state stored, got err: %v", err)
	}
}
