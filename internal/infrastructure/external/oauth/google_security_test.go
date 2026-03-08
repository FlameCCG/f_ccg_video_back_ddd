package oauth

import (
	"f_ccg_video_back_ddd/internal/config"
	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGooglePreservesProviderEmailVerificationAssertion(t *testing.T) {
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
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}
		info, err := client.FetchUser(t.Context(), "code")
		if err != nil {
			t.Fatal(err)
		}
		if info.EmailVerified != verified {
			t.Fatalf("email verified=%t want=%t", info.EmailVerified, verified)
		}
	}
}
