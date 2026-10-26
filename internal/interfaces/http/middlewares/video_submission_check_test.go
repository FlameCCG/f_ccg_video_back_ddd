package middlewares

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/pkg/constants"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/gin-gonic/gin"
)

type submissionRoles struct {
	identityapp.UserRoleQuery
	items []identityapp.UserRoleItem
	err   error
}

func (r *submissionRoles) LoadRoles(context.Context, uint) ([]identityapp.UserRoleItem, error) {
	return r.items, r.err
}

func TestVideoSubmissionCheck(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		cfg   *site.Site
		uid   string
		roles *submissionRoles
		want  string
	}{
		{"legacy config", nil, "7", nil, "allowed"},
		{"disabled switch", &site.Site{}, "7", nil, "allowed"},
		{"anonymous", &site.Site{}, "", nil, "请先登录"},
		{"regular user", &site.Site{DisableUserSubmission: true}, "7", &submissionRoles{}, "网站已暂停用户投稿"},
		{"other role", &site.Site{DisableUserSubmission: true}, "7", &submissionRoles{items: []identityapp.UserRoleItem{{ID: 1, Name: "admin"}}}, "网站已暂停用户投稿"},
		{"administrator", &site.Site{DisableUserSubmission: true}, "7", &submissionRoles{items: []identityapp.UserRoleItem{{Name: constants.VipUserRole}, {Name: constants.AdminUserRole}}}, "allowed"},
		{"query failure", &site.Site{DisableUserSubmission: true}, "7", &submissionRoles{err: errors.New("unavailable")}, "投稿权限验证失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := &Middlewares{cfgProvider: infraconfig.NewProvider(&config.Config{Site: tc.cfg})}
			if tc.roles != nil {
				m.userRoles = tc.roles
			}

			engine := gin.New()
			engine.POST("/publish", func(c *gin.Context) { c.Set("uid", tc.uid) }, m.VideoSubmissionCheck(), func(c *gin.Context) { c.String(http.StatusOK, "allowed") })

			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/publish", strings.NewReader(`{"isAdmin":true}`)))

			require.Contains(t, rec.Body.String(), tc.want, "got %s, want %s", rec.Body.String(), tc.want)
		})
	}
}

func TestVideoSubmissionCheckReloadsConfigAndRoles(t *testing.T) {
	t.Parallel()

	roles := &submissionRoles{}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{}})
	m := &Middlewares{cfgProvider: provider, userRoles: roles}
	engine := gin.New()
	engine.POST("/publish", func(c *gin.Context) { c.Set("uid", "7") }, m.VideoSubmissionCheck(), func(c *gin.Context) { c.String(http.StatusOK, "allowed") })

	check := func(want string) {
		t.Helper()

		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/publish", nil))

		require.Contains(t, rec.Body.String(), want, "got %s, want %s", rec.Body.String(), want)
	}
	check("allowed")
	provider.Set(&config.Config{Site: &site.Site{DisableUserSubmission: true}})
	check("网站已暂停用户投稿")

	roles.items = []identityapp.UserRoleItem{{Name: constants.AdminUserRole}}

	check("allowed")

	roles.items = nil

	check("网站已暂停用户投稿")
	provider.Set(&config.Config{Site: &site.Site{}})
	check("allowed")
}
