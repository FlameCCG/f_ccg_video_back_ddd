package middlewares

import (
	"context"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/captcha"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	captcha_service "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/gin-gonic/gin"
)

const (
	captchaClick = "click"
	captchaSlide = "slide"
)

type stubInteractiveCaptchaVerifier struct {
	calls string
	ok    bool
	err   error
}

func (s *stubInteractiveCaptchaVerifier) VerifyClick(_ context.Context, _ string, _ []captcha_service.ClickCaptchaPoint) (bool, error) {
	s.calls += captchaClick
	return s.ok, s.err
}

func (s *stubInteractiveCaptchaVerifier) VerifySlide(_ context.Context, _ captcha_service.SlideCaptchaVerifyReq) (bool, error) {
	s.calls += captchaSlide
	return s.ok, s.err
}

func TestInteractiveCaptchaVerify(t *testing.T) {
	t.Parallel()

	const (
		click = `{"captchaToken":"click-token","captchaDots":[{"index":0,"x":20,"y":30}]}`
		slide = `{"slideCaptchaToken":"slide-token","slideCaptchaX":100,"slideCaptchaY":30}`
	)

	tests := []struct {
		name      string
		options   site.CaptchaOptions
		body      string
		ok        bool
		err       error
		wantPass  bool
		wantCalls string
	}{
		{"disabled", site.CaptchaOptions{}, "{}", false, nil, true, ""},
		{"click only", site.CaptchaOptions{Click: true}, click, true, nil, true, captchaClick},
		{"slide only", site.CaptchaOptions{Slide: true}, slide, true, nil, true, captchaSlide},
		{"both accepts click", site.CaptchaOptions{Click: true, Slide: true}, click, true, nil, true, captchaClick},
		{"both accepts slide", site.CaptchaOptions{Click: true, Slide: true}, slide, true, nil, true, captchaSlide},
		{"requires captcha", site.CaptchaOptions{Click: true, Slide: true}, "{}", true, nil, false, ""},
		{"reject disabled click", site.CaptchaOptions{Slide: true}, click, true, nil, false, ""},
		{"reject disabled slide", site.CaptchaOptions{Click: true}, slide, true, nil, false, ""},
		{"reject empty points", site.CaptchaOptions{Click: true}, `{"captchaToken":"token"}`, true, nil, false, ""},
		{"reject both payloads", site.CaptchaOptions{Click: true, Slide: true}, `{"captchaToken":"click","captchaDots":[{"x":1}],"slideCaptchaToken":"slide"}`, true, nil, false, ""},
		{"wrong answer", site.CaptchaOptions{Slide: true}, slide, false, nil, false, captchaSlide},
		{"expired", site.CaptchaOptions{Click: true}, click, false, captcha.ErrCaptchaExpired, false, captchaClick},
		{"malformed json", site.CaptchaOptions{Click: true}, "{", true, nil, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			verifier := &stubInteractiveCaptchaVerifier{ok: tt.ok, err: tt.err}
			router := gin.New()
			passed := false

			router.POST("/", interactiveCaptchaVerify(func() site.CaptchaOptions { return tt.options }, verifier), func(c *gin.Context) {
				passed = true

				raw, _ := io.ReadAll(c.Request.Body)
				assert.Equal(t, string(raw), tt.body, "request body was not restored: %s", raw)

				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.False(t, passed != tt.wantPass || verifier.calls != tt.wantCalls, "passed=%v calls=%q, want passed=%v calls=%q; response=%s", passed, verifier.calls, tt.wantPass, tt.wantCalls, recorder.Body.String())

			assert.False(t, tt.err != nil && !strings.Contains(recorder.Body.String(), tt.err.Error()), "response lost expiry error: %s", recorder.Body.String())
		})
	}
}
