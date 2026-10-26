package oauth

import (
	"f_ccg_video_back_ddd/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

func TestGooglePreservesProviderEmailVerificationAssertion(t *testing.T) {
	t.Parallel()

	for _, verified := range []bool{false, true} {
		provider := infraconfig.NewProvider(&config.Config{ThirdLogin: &config.ThirdLogin{Google: &thirdlogin.Google{ClientID: "id", ClientSecret: "secret"}}})
		client := NewGoogleClient(provider, zap.NewNop())
		client.httpClient = &http.Client{Transport: githubRoundTrip(func(r *http.Request) (*http.Response, error) {
			body := `{"access_token":"upstream-token"}`
			if r.URL.String() == googleUserInfoURL {
				body = `{"sub":"99","email":"owner@example.com","email_verified":false}`
				if verified {
					body = strings.Replace(body, ":false", ":true", 1)
				}
			}

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}

		info, err := client.FetchUser(t.Context(), "code")
		require.NoError(t, err)

		require.Equal(t, info.EmailVerified, verified, "email verified=%t want=%t", info.EmailVerified, verified)
	}
}
