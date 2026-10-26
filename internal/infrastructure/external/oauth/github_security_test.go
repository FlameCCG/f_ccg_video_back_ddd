package oauth

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
)

type githubRoundTrip func(*http.Request) (*http.Response, error)

func (f githubRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGitHubOnlyTrustsVerifiedEmailEndpoint(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, test := range []struct{ name, emails, want string }{
		{"unverified primary rejected", `[{"email":"victim@example.com","primary":true,"verified":false}]`, ""},
		{"verified secondary wins", `[{"email":"victim@example.com","primary":true,"verified":false},{"email":"owner@example.com","primary":false,"verified":true}]`, "owner@example.com"},
		{"verified primary wins", `[{"email":"secondary@example.com","primary":false,"verified":true},{"email":"owner@example.com","primary":true,"verified":true}]`, "owner@example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := infraconfig.NewProvider(&config.Config{ThirdLogin: &config.ThirdLogin{GitHub: &thirdlogin.GitHub{ClientID: "id", ClientSecret: "secret"}}})
			client := NewGitHubClient(provider)
			queriedEmails := false
			client.httpClient = &http.Client{Transport: githubRoundTrip(func(r *http.Request) (*http.Response, error) {
				body := `{"access_token":"upstream-token"}`

				switch r.URL.String() {
				case githubUserInfoURL:
					body = `{"id":99,"email":"victim@example.com"}`
				case githubUserEmailsURL:
					body = test.emails
					queriedEmails = true
				}

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}

			info, err := client.FetchUser(context.Background(), "code", "")
			require.NoError(t, err)

			require.True(t, queriedEmails, "email=%q want=%q queriedVerifiedEndpoint=%t", info.Email, test.want, queriedEmails)
			require.Equal(t, test.want, info.Email, "email=%q want=%q queriedVerifiedEndpoint=%t", info.Email, test.want, queriedEmails)
			require.Equal(t, info.EmailVerified, (test.want != ""), "email=%q want=%q queriedVerifiedEndpoint=%t", info.Email, test.want, queriedEmails)
		})
	}
}
