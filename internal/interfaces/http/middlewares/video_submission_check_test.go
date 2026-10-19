package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/constants"

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
	gin.SetMode(gin.TestMode)
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
			m := &Middlewares{cfgProvider: infraconfig.NewProvider(&config.Config{Site: tc.cfg})}
			if tc.roles != nil {
				m.userRoles = tc.roles
			}
			engine := gin.New()
			engine.POST("/publish", func(c *gin.Context) { c.Set("uid", tc.uid) }, m.VideoSubmissionCheck(), func(c *gin.Context) { c.String(http.StatusOK, "allowed") })
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/publish", strings.NewReader(`{"isAdmin":true}`)))
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("got %s, want %s", rec.Body.String(), tc.want)
			}
		})
	}
}

func TestVideoSubmissionCheckReloadsConfigAndRoles(t *testing.T) {
	roles := &submissionRoles{}
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{}})
	m := &Middlewares{cfgProvider: provider, userRoles: roles}
	engine := gin.New()
	engine.POST("/publish", func(c *gin.Context) { c.Set("uid", "7") }, m.VideoSubmissionCheck(), func(c *gin.Context) { c.String(http.StatusOK, "allowed") })
	check := func(want string) {
		t.Helper()
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/publish", nil))
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("got %s, want %s", rec.Body.String(), want)
		}
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
