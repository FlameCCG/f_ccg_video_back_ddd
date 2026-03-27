package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"github.com/gin-gonic/gin"
)

func TestLoginErrorsDistinguishSessionDependencyFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"storage", fmt.Errorf("%w: %w", app.ErrSessionIssueFailed, errors.New("private Redis address")), 503, "身份验证暂时不可用"},
		{"timeout", fmt.Errorf("%w: %w", app.ErrSessionIssueFailed, context.DeadlineExceeded), 504, "身份验证超时"},
		{"credentials", domain.ErrCredentialsInvalid, 200, domain.ErrCredentialsInvalid.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, ctx := newTestContext("POST", "/v1/admin/user/login/pwd")
			writeLoginError(ctx, tc.err)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.message) || strings.Contains(rec.Body.String(), "private") {
				t.Fatalf("response=%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
