package oauth

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"io"
	"net/http"
	"strings"
	"testing"
)

type githubRoundTrip func(*http.Request) (*http.Response, error)

func (f githubRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGitHubOnlyTrustsVerifiedEmailEndpoint(t *testing.T) {
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
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			info, err := client.FetchUser(context.Background(), "code", "")
			if err != nil {
				t.Fatal(err)
			}
			if !queriedEmails || info.Email != test.want || info.EmailVerified != (test.want != "") {
				t.Fatalf("email=%q want=%q queriedVerifiedEndpoint=%t", info.Email, test.want, queriedEmails)
			}
		})
	}
}
