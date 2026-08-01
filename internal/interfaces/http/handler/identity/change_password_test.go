package identity

import (
	"net/http"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/encrypt"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newChangePasswordHandler 构造只装配账号凭据用例的处理器（注入式，不依赖全局状态）。
func newChangePasswordHandler(t *testing.T) (*Handler, *gorm.DB) {
	t.Helper()

	db := openTestDB(t, &po.User{}, &po.UserConf{}, &po.FavoriteFolder{})
	users := persistence.NewUserRepository(db, nil)
	accounts := app.NewAccountService(users, nil)

	return NewHandler(nil, nil, nil, nil, accounts, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop()), db
}

// insertChangePasswordTestUser 落库一个用户（跳过钩子，本测无需 user_confs 副作用）。
func insertChangePasswordTestUser(t *testing.T, db *gorm.DB, user po.User) {
	t.Helper()

	if err := db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
}

// performChangePassword 调用 ChangePasswordView 并返回响应体。
func performChangePassword(t *testing.T, handler *Handler, req ChangePasswordReq, uid string) res.Response {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/password/change")
	ctx.Set("request", req)
	ctx.Set("uid", uid)

	handler.ChangePasswordView(ctx)

	return decodeResponse(t, recorder)
}

func TestChangePasswordViewRequiresOldPasswordForPasswordUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, db := newChangePasswordHandler(t)

	oldHash, err := encrypt.GenPasswordHash([]byte("old-password"))
	if err != nil {
		t.Fatalf("generate old password hash failed: %v", err)
	}

	insertChangePasswordTestUser(t, db, po.User{
		Model:    po.Model{ID: 1},
		Username: "pwd_user",
		Email:    "pwd@example.com",
		Password: string(oldHash),
	})

	response := performChangePassword(t, handler, ChangePasswordReq{
		OldPassword: "",
		NewPassword: "new-password",
	}, "1")

	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "请输入旧密码" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

func TestChangePasswordViewRejectsWrongOldPasswordForPasswordUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, db := newChangePasswordHandler(t)

	oldHash, err := encrypt.GenPasswordHash([]byte("old-password"))
	if err != nil {
		t.Fatalf("generate old password hash failed: %v", err)
	}

	insertChangePasswordTestUser(t, db, po.User{
		Model:    po.Model{ID: 2},
		Username: "wrong_old_pwd_user",
		Email:    "wrong-old@example.com",
		Password: string(oldHash),
	})

	response := performChangePassword(t, handler, ChangePasswordReq{
		OldPassword: "wrong-password",
		NewPassword: "new-password",
	}, "2")

	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "旧密码错误" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

func TestChangePasswordViewSkipsOldPasswordForOAuthUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, db := newChangePasswordHandler(t)

	insertChangePasswordTestUser(t, db, po.User{
		Model:          po.Model{ID: 3},
		Username:       "oauth_user",
		Email:          "oauth@example.com",
		Password:       "",
		OpenID:         "oauth-open-id",
		RegisterSource: "google",
	})

	response := performChangePassword(t, handler, ChangePasswordReq{
		NewPassword: "new-password",
	}, "3")

	if response.Code != res.SuccessCode {
		t.Fatalf("expected success response code, got %d", response.Code)
	}

	if response.Msg != "修改成功" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}

	var user po.User
	if err := db.First(&user, "id = ?", 3).Error; err != nil {
		t.Fatalf("query updated user failed: %v", err)
	}

	if !encrypt.ValidatePasswordHash("new-password", user.Password) {
		t.Fatalf("expected password to be updated with bcrypt hash")
	}
}

func TestChangePasswordViewRequiresOldPasswordAfterOAuthUserSetsPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler, db := newChangePasswordHandler(t)

	oldHash, err := encrypt.GenPasswordHash([]byte("old-password"))
	if err != nil {
		t.Fatalf("generate old password hash failed: %v", err)
	}

	// 第三方账号已设密后，再次修改必须校验旧密码
	insertChangePasswordTestUser(t, db, po.User{
		Model:          po.Model{ID: 4},
		Username:       "oauth_with_pwd",
		Email:          "oauth-pwd@example.com",
		Password:       string(oldHash),
		OpenID:         "oauth-open-id-2",
		RegisterSource: "google",
	})

	response := performChangePassword(t, handler, ChangePasswordReq{
		OldPassword: "",
		NewPassword: "new-password",
	}, "4")

	if response.Code != res.FailServiceCode {
		t.Fatalf("expected fail response code, got %d", response.Code)
	}

	if response.Msg != "请输入旧密码" {
		t.Fatalf("unexpected response msg: %s", response.Msg)
	}
}

// TestUserPasswordFlags 迁移原 user_api 的同名用例：
// 判定逻辑已上移到领域层（domain.User 的 HasPassword / IsThirdParty）。
func TestUserPasswordFlags(t *testing.T) {
	if (&domain.User{Password: ""}).HasPassword() {
		t.Fatal("empty password should be hasPassword=false")
	}

	if !(&domain.User{Password: "hash"}).HasPassword() {
		t.Fatal("non-empty password should be hasPassword=true")
	}

	if (&domain.User{OpenID: ""}).IsThirdParty() {
		t.Fatal("empty open id should be isThirdParty=false")
	}

	if !(&domain.User{OpenID: "abc"}).IsThirdParty() {
		t.Fatal("non-empty open id should be isThirdParty=true")
	}
}
