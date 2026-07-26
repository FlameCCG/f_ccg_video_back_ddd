package contentops

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"f_ccg_video_back_ddd/internal/infrastructure/config"
	thirdlogin "f_ccg_video_back_ddd/internal/infrastructure/config/thirdlogin"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

func TestGoogleUrlViewReturnsAuthURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, _ := newOAuthHandler(t, &config.Config{
		ThirdLogin: &config.ThirdLogin{
			Google: &thirdlogin.Google{
				ClientID:     testGoogleClientID,
				ClientSecret: "google-client-secret",
				Redirect:     testGoogleRedirect,
			},
		},
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/site/google-url", nil)

	handler.GoogleUrlView(ctx)

	if recorder.Code != 200 {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	authURL, ok := response.Data.(string)
	if !ok {
		t.Fatalf("unexpected auth url type: %T", response.Data)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth url failed: %v", err)
	}

	if parsed.Host != "accounts.google.com" {
		t.Fatalf("unexpected google auth host: %s", parsed.Host)
	}

	query := parsed.Query()
	if query.Get("client_id") != testGoogleClientID {
		t.Fatalf("unexpected client_id: %s", query.Get("client_id"))
	}

	if query.Get("redirect_uri") != testGoogleRedirect {
		t.Fatalf("unexpected redirect_uri: %s", query.Get("redirect_uri"))
	}

	if query.Get("scope") != "openid profile email" {
		t.Fatalf("unexpected scope: %s", query.Get("scope"))
	}
}
