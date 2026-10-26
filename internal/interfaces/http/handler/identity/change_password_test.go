package identity

import (
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/encrypt"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	testNewPassword = "new-password"
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

	{
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "insert user failed: %v", err)
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
	t.Parallel()
	assertOldPasswordRejected(t, po.User{Model: po.Model{ID: 1}, Username: "pwd_user", Email: "pwd@example.com"}, "", "请输入旧密码")
}

func TestChangePasswordViewRejectsWrongOldPasswordForPasswordUser(t *testing.T) {
	t.Parallel()
	assertOldPasswordRejected(t, po.User{Model: po.Model{ID: 2}, Username: "wrong_old_pwd_user", Email: "wrong-old@example.com"}, "wrong-password", "旧密码错误")
}

func assertOldPasswordRejected(t *testing.T, user po.User, supplied, expected string) {
	t.Helper()
	handler, db := newChangePasswordHandler(t)
	hash, err := encrypt.GenPasswordHash([]byte("old-password"))
	require.NoError(t, err)

	user.Password = string(hash)
	insertChangePasswordTestUser(t, db, user)
	response := performChangePassword(t, handler, ChangePasswordReq{OldPassword: supplied, NewPassword: testNewPassword}, strconv.FormatUint(uint64(user.ID), 10))
	require.EqualValues(t, res.FailServiceCode, response.Code)
	require.Equal(t, expected, response.Msg)
}

func TestChangePasswordViewSkipsOldPasswordForOAuthUser(t *testing.T) {
	t.Parallel()

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
		NewPassword: testNewPassword,
	}, "3")

	require.EqualValues(t, res.SuccessCode, response.Code, "expected success response code, got %d", response.Code)

	require.Equal(t, "修改成功", response.Msg, "unexpected response msg: %s", response.Msg)

	var user po.User
	{
		err := db.First(&user, "id = ?", 3).Error
		require.NoError(t, err, "query updated user failed: %v", err)
	}

	require.True(t, encrypt.ValidatePasswordHash(testNewPassword, user.Password), "expected password to be updated with bcrypt hash")
}

func TestChangePasswordViewRequiresOldPasswordAfterOAuthUserSetsPassword(t *testing.T) {
	t.Parallel()

	handler, db := newChangePasswordHandler(t)

	oldHash, err := encrypt.GenPasswordHash([]byte("old-password"))
	require.NoError(t, err, "generate old password hash failed: %v", err)

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
		NewPassword: testNewPassword,
	}, "4")

	require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

	require.Equal(t, "请输入旧密码", response.Msg, "unexpected response msg: %s", response.Msg)
}

// TestUserPasswordFlags 迁移原 user_api 的同名用例：
// 判定逻辑已上移到领域层（domain.User 的 HasPassword / IsThirdParty）。
func TestUserPasswordFlags(t *testing.T) {
	t.Parallel()

	require.False(t, (&domain.User{Password: ""}).HasPassword(), "empty password should be hasPassword=false")

	require.True(t, (&domain.User{Password: "hash"}).HasPassword(), "non-empty password should be hasPassword=true")

	require.False(t, (&domain.User{OpenID: ""}).IsThirdParty(), "empty open id should be isThirdParty=false")

	require.True(t, (&domain.User{OpenID: "abc"}).IsThirdParty(), "non-empty open id should be isThirdParty=true")
}
