package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	identityhandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/identity"
	videohandler "f_ccg_video_back_ddd/internal/interfaces/http/handler/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/constants"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"f_ccg_video_back_ddd/pkg/validate"

	"github.com/gin-gonic/gin"
)

type submissionStatusCache struct {
	identitydomain.UserStatusCache
	status enum.UserStatus
}

func (s *submissionStatusCache) Load(context.Context, uint) (enum.UserStatus, bool) {
	return s.status, true
}

type routeSubmissionUsers struct{ identitydomain.UserRepository }

func (*routeSubmissionUsers) FindByID(context.Context, uint) (*identitydomain.User, error) {
	return &identitydomain.User{ID: 7, Username: "admin", Status: enum.UserStatusActive}, nil
}

type routeSubmissionRoles struct {
	identityapp.UserRoleQuery
	admin bool
}

func (r *routeSubmissionRoles) LoadRoles(context.Context, uint) ([]identityapp.UserRoleItem, error) {
	if r.admin {
		return []identityapp.UserRoleItem{{Name: constants.AdminUserRole}}, nil
	}
	return nil, nil
}

func TestVideoSubmissionRoutesEnforcePolicyBeforeBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "submission-route-test-secret-32-characters"
	provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{DisableUserSubmission: true}, Jwt: &config.Jwt{AccessSecret: secret}})
	roles := &routeSubmissionRoles{}
	statuses := &submissionStatusCache{status: enum.UserStatusActive}
	mw := middlewares.New(provider, nil, identityapp.NewUserStatusService(statuses, &routeSubmissionUsers{}), roles, nil, nil, nil, nil, nil)
	engine := gin.New()
	engine.Use(validate.New().Middleware())
	uploads := videoapp.NewUploadService(&submissionStorage{}, provider, nil, nil, nil)
	handler := videohandler.NewHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, uploads, nil, nil, nil, nil, provider, nil, nil, identityapp.NewUserStatusService(statuses, &routeSubmissionUsers{}))
	registerVideoCommonRoutes(engine.Group("/v1/common"), mw, handler)
	registerIdentityCommonRoutes(engine.Group("/v1/common"), mw, &identityhandler.Handler{})
	token, err := jwts.CreateJwtToken("7", "admin", time.Hour, secret, time.Hour, secret)
	if err != nil {
		t.Fatal(err)
	}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/video/publish"},
		{http.MethodPost, "/user/video/tag"},
		{http.MethodPut, "/video/publish"},
		{http.MethodPost, "/video/upload/chunk"},
		{http.MethodGet, "/video/upload/status"},
		{http.MethodPost, "/video/upload/complete"},
		{http.MethodPost, "/video/image/upload"},
	}
	request := func(method, path string) string {
		t.Helper()
		req := httptest.NewRequest(method, "/v1/common"+path, strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("missing route: %s", path)
		}
		return rec.Body.String()
	}
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			roles.admin = false
			if body := request(route.method, route.path); !strings.Contains(body, "网站已暂停用户投稿") {
				t.Fatalf("regular user was not blocked: %s", body)
			}
			roles.admin = true
			if body := request(route.method, route.path); strings.Contains(body, "投稿权限") || strings.Contains(body, "网站已暂停") {
				t.Fatalf("admin was blocked: %s", body)
			}
		})
	}
	roles.admin = false
	if body := request(http.MethodPost, "/image/upload"); strings.Contains(body, "网站已暂停") {
		t.Fatalf("general image upload was blocked: %s", body)
	}
	roles.admin = true
	statuses.status = enum.UserStatusPermanentlyBanned
	if body := request(http.MethodPost, "/video/publish"); !strings.Contains(body, "永久封禁") {
		t.Fatalf("banned administrator was allowed: %s", body)
	}
}

type submissionStorage struct{ videoapp.StorageResolver }

func (*submissionStorage) ValidateUploadID(string) error { return errors.New("图片标识无效") }
