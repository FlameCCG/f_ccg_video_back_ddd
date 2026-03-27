package identity

import (
	"context"
	"errors"
	"fmt"

	"f_ccg_video_back_ddd/internal/application/shared"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/encrypt"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"go.uber.org/zap"
)

// 登录记录里的登录方式标识。
const (
	loginTypePwd      = "pwd"
	loginTypeAdminPwd = "admin_pwd"
)

// LoginService 密码登录与令牌刷新用例服务。
type LoginService struct {
	cfgProvider  shared.ConfigReader
	users        identitydomain.UserAccountRepository
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
	users identitydomain.UserAccountRepository,
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

	hasRole, err := s.roles.HasRole(ctx, user.ID(), adminRoleID)
	if err != nil {
		s.logger.Error("check admin role error", zap.Error(err))

		return nil, ErrRoleCheckFailed
	}

	if err := user.EnsureAdminLogin(hasRole); err != nil {
		return nil, err
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
	if err != nil {
		s.logger.Error("query refresh-token user error", zap.Uint("user_id", userID), zap.Error(err))

		return nil, fmt.Errorf("%w: %w", ErrQueryUserFailed, err)
	}

	if user == nil {
		return nil, identitydomain.ErrUserNotFound
	}

	if err := user.EnsureCanLogin(); err != nil {
		return nil, err
	}

	token, err := s.tokens.Rotate(ctx, user, refreshToken)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenReused) {
			s.logger.Warn("refresh token reuse detected; session revoked", zap.Uint("user_id", user.ID()))
		} else {
			s.logger.Warn("refresh token rotation failed", zap.Error(err))
		}
		return nil, err
	}

	return token, nil
}

// authenticate 校验账号密码与封禁状态；logScope 用于区分前台/后台登录日志。
func (s *LoginService) authenticate(ctx context.Context, in PwdLoginInput, logScope string) (*identitydomain.User, error) {
	user, err := s.users.FindByUsernameOrEmail(ctx, in.Identifier)
	if err != nil {
		s.logger.Error("query user error", zap.Error(err))

		return nil, ErrQueryUserFailed
	}

	if user == nil {
		s.logger.Warn(logScope+" user not found", zap.String("username", in.Identifier))

		return nil, user.EnsurePasswordLogin(false)
	}

	passwordMatches := encrypt.ValidatePasswordHash(in.Password, user.Password())
	if err := user.EnsurePasswordLogin(passwordMatches); err != nil {
		if !passwordMatches {
			s.logger.Warn(logScope+" password error", zap.String("username", in.Identifier))
		}
		return nil, err
	}

	return user, nil
}

// finishLogin 兜底创建默认收藏夹、异步记录登录 IP，并签发令牌。
func (s *LoginService) finishLogin(
	ctx context.Context,
	user *identitydomain.User,
	in PwdLoginInput,
	loginType string,
) (*jwts.JwtToken, error) {
	// 确保默认收藏夹存在。
	if s.folders != nil {
		_ = s.folders.EnsureDefaultFolder(ctx, user.ID())
	}

	s.RecordLoginAsync(ctx, user.ID(), in.IP, in.Addr, in.UserAgent, loginType)

	token, err := s.tokens.IssueForUser(ctx, user)
	if err != nil {
		s.logger.Error("create jwt token failed", zap.Error(err))

		return nil, fmt.Errorf("%w: %w", ErrSessionIssueFailed, err)
	}

	return token, nil
}

// RecordLoginAsync 以 detached 任务写登录记录。
func (s *LoginService) RecordLoginAsync(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) {
	(asyncLoginLog{tasks: s.tasks, records: s.loginRecords, logger: s.logger}).write(ctx, userID, ip, addr, userAgent, loginType)
}

// asyncLoginLog 把登录记录写入放到 detached 任务里。
//
// 密码登录与 OAuth 登录共用，避免服务之间互相依赖。
type asyncLoginLog struct {
	tasks   *ctxutil.DetachedTaskGroup
	records *LoginRecordService
	logger  *zap.Logger
}

func (l asyncLoginLog) write(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) {
	if l.tasks == nil || l.records == nil {
		return
	}

	l.tasks.Go(ctx, ctxutil.AsyncQuickTimeout, func(taskCtx context.Context) {
		if err := l.records.Record(taskCtx, userID, ip, addr, userAgent, loginType); err != nil {
			l.logger.Warn("记录登录IP失败", zap.Error(err))
		}
	})
}

func (s *LoginService) usernamePwdLoginEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.UsernamePwdLogin
}
