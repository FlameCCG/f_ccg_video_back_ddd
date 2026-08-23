package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/application/shared"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/encrypt"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"go.uber.org/zap"
)

// 登录记录里的登录方式标识（与原实现逐字一致）。
const (
	loginTypePwd      = "pwd"
	loginTypeAdminPwd = "admin_pwd"
)

// LoginService 密码登录与令牌刷新用例服务
// （原 api/user_api/pwd_login.go、admin_pwd_login.go、refresh_token.go）。
type LoginService struct {
	cfgProvider  shared.ConfigReader
	users        domain.UserAccountRepository
	roles        UserRoleQuery
	tokens       *TokenService
	folders      DefaultFolderEnsurer
	loginRecords *LoginRecordService
	tasks        *ctxutil.DetachedTaskGroup
	logger       *zap.Logger
}

// NewLoginService 构造密码登录服务。
func NewLoginService(
	cfgProvider shared.ConfigReader,
	users domain.UserAccountRepository,
	roles UserRoleQuery,
	tokens *TokenService,
	folders DefaultFolderEnsurer,
	loginRecords *LoginRecordService,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *LoginService {
	return &LoginService{
		cfgProvider:  cfgProvider,
		users:        users,
		roles:        roles,
		tokens:       tokens,
		folders:      folders,
		loginRecords: loginRecords,
		tasks:        tasks,
		logger:       logger,
	}
}

// PwdLogin 用户名/邮箱 + 密码登录。
func (s *LoginService) PwdLogin(ctx context.Context, in PwdLoginInput) (*jwts.JwtToken, error) {
	if !s.usernamePwdLoginEnabled() {
		return nil, ErrPwdLoginDisabled
	}

	user, err := s.authenticate(ctx, in, "login")
	if err != nil {
		return nil, err
	}

	return s.finishLogin(ctx, user, in, loginTypePwd)
}

// AdminPwdLogin 管理端用户名/邮箱 + 密码登录（额外校验超级管理员角色）。
func (s *LoginService) AdminPwdLogin(ctx context.Context, in PwdLoginInput) (*jwts.JwtToken, error) {
	if !s.usernamePwdLoginEnabled() {
		return nil, ErrPwdLoginDisabled
	}

	user, err := s.authenticate(ctx, in, "admin login")
	if err != nil {
		return nil, err
	}

	adminRoleID, err := s.roles.RoleIDByName(ctx, constants.AdminUserRole)
	if err != nil {
		s.logger.Error("query admin role error", zap.Error(err))

		return nil, ErrAdminRoleMissing
	}

	hasRole, err := s.roles.HasRole(ctx, user.ID, adminRoleID)
	if err != nil {
		s.logger.Error("check admin role error", zap.Error(err))

		return nil, ErrRoleCheckFailed
	}

	if !hasRole {
		return nil, ErrNotAdminAccount
	}

	return s.finishLogin(ctx, user, in, loginTypeAdminPwd)
}

// RefreshToken 用 refresh 令牌换取新的令牌对。
func (s *LoginService) RefreshToken(ctx context.Context, refreshToken string) (*jwts.JwtToken, error) {
	userID, err := s.tokens.ParseRefresh(refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		// 原实现对查询失败与用户不存在都回「用户不存在」。
		return nil, domain.ErrUserNotFound
	}

	if user.IsPermanentlyBanned() {
		return nil, domain.ErrAccountPermanentlyBanned
	}

	token, err := s.tokens.Issue(user.ID, user.Username)
	if err != nil {
		return nil, ErrRefreshTokenFailed
	}

	return token, nil
}

// authenticate 校验账号密码与封禁状态；logScope 用于区分前台/后台登录日志。
func (s *LoginService) authenticate(ctx context.Context, in PwdLoginInput, logScope string) (*domain.User, error) {
	user, err := s.users.FindByUsernameOrEmail(ctx, in.Identifier)
	if err != nil {
		s.logger.Error("query user error", zap.Error(err))

		return nil, ErrQueryUserFailed
	}

	if user == nil {
		s.logger.Warn(logScope+" user not found", zap.String("username", in.Identifier))

		return nil, domain.ErrCredentialsInvalid
	}

	if !encrypt.ValidatePasswordHash(in.Password, user.Password) {
		s.logger.Warn(logScope+" password error", zap.String("username", in.Identifier))

		return nil, domain.ErrCredentialsInvalid
	}

	if user.IsPermanentlyBanned() {
		return nil, domain.ErrAccountPermanentlyBanned
	}

	return user, nil
}

// finishLogin 兜底创建默认收藏夹、异步记录登录 IP，并签发令牌。
func (s *LoginService) finishLogin(
	ctx context.Context,
	user *domain.User,
	in PwdLoginInput,
	loginType string,
) (*jwts.JwtToken, error) {
	// 确保默认收藏夹存在（与原实现一致：忽略错误）。
	if s.folders != nil {
		_ = s.folders.EnsureDefaultFolder(ctx, user.ID)
	}

	s.RecordLoginAsync(ctx, user.ID, in.IP, in.Addr, in.UserAgent, loginType)

	token, err := s.tokens.Issue(user.ID, user.Username)
	if err != nil {
		s.logger.Error("create jwt token failed", zap.Error(err))

		return nil, ErrLoginFailed
	}

	return token, nil
}

// RecordLoginAsync 以 detached 任务写登录记录（原 api/user_api/login_async.go）。
func (s *LoginService) RecordLoginAsync(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) {
	recordLoginAsync(ctx, s.tasks, s.loginRecords, userID, ip, addr, userAgent, loginType, s.logger)
}

// recordLoginAsync 把登录记录写入放到 detached 任务里（原 ctxutil.GoDetached + AsyncQuickTimeout）。
//
// 密码登录与 OAuth 登录共用：写成包级纯函数而非共享状态，避免服务之间互相依赖。
func recordLoginAsync(
	ctx context.Context,
	tasks *ctxutil.DetachedTaskGroup,
	records *LoginRecordService,
	userID uint,
	ip, addr, userAgent, loginType string,
	logger *zap.Logger,
) {
	if tasks == nil || records == nil {
		return
	}

	tasks.Go(ctx, ctxutil.AsyncQuickTimeout, func(taskCtx context.Context) {
		if err := records.Record(taskCtx, userID, ip, addr, userAgent, loginType); err != nil {
			logger.Warn("记录登录IP失败", zap.Error(err))
		}
	})
}

func (s *LoginService) usernamePwdLoginEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.UsernamePwdLogin
}
