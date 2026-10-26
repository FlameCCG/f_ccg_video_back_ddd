package contentops

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"

	"github.com/gin-gonic/gin"
)

func TestGoogleUrlViewReturnsAuthURL(t *testing.T) {
	t.Parallel()

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

	handler.GoogleURLView(ctx)

	require.Equal(t, 200, recorder.Code, "unexpected status code: %d", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	authURL, ok := response.Data.(string)
	require.True(t, ok, "unexpected auth url type: %T", response.Data)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err, "parse auth url failed: %v", err)

	require.Equal(t, "accounts.google.com", parsed.Host, "unexpected google auth host: %s", parsed.Host)

	query := parsed.Query()
	require.Equal(t, testGoogleClientID, query.Get("client_id"), "unexpected client_id: %s", query.Get("client_id"))

	require.Equal(t, testGoogleRedirect, query.Get("redirect_uri"), "unexpected redirect_uri: %s", query.Get("redirect_uri"))

	require.Equal(t, "openid profile email", query.Get("scope"), "unexpected scope: %s", query.Get("scope"))
}
