package identity

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"fmt"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"go.uber.org/zap"
)

// 第三方登录提供商标识（同时作为 users.register_source 的取值，属于外部契约）。
const (
	// OAuthProviderQQ QQ 登录。
	OAuthProviderQQ = "qq"
	// OAuthProviderGoogle Google 登录。
	OAuthProviderGoogle = "google"
	// OAuthProviderGitHub GitHub 登录。
	OAuthProviderGitHub = "github"
	// OAuthProviderLinuxDo Linux.do 登录。
	OAuthProviderLinuxDo = "linuxdo"
	// OAuthProviderX X (Twitter) 登录。
	OAuthProviderX = "x"
)

// OAuthLoginService 第三方登录用例服务
// （原 api/user_api/{qq,google,github,linuxdo,x}_login.go 与 oauth_login_helper.go）。
type OAuthLoginService struct {
	cfgProvider  shared.ConfigReader
	txm          tx.Manager
	users        domain.UserAccountRepository
	roles        UserRoleQuery
	binder       UserRoleBinder
	states       OAuthStateStore
	tokens       *TokenService
	loginRecords *LoginRecordService
	tasks        *ctxutil.DetachedTaskGroup
	qq           QQOAuthClient
	google       GoogleOAuthClient
	github       GitHubOAuthClient
	linuxdo      LinuxDoOAuthClient
	x            XOAuthClient
	logger       *zap.Logger
}

// NewOAuthLoginService 构造第三方登录服务。
func NewOAuthLoginService(
	cfgProvider shared.ConfigReader,
	txm tx.Manager,
	users domain.UserAccountRepository,
	roles UserRoleQuery,
	binder UserRoleBinder,
	states OAuthStateStore,
	tokens *TokenService,
	loginRecords *LoginRecordService,
	tasks *ctxutil.DetachedTaskGroup,
	qq QQOAuthClient,
	google GoogleOAuthClient,
	github GitHubOAuthClient,
	linuxdo LinuxDoOAuthClient,
	x XOAuthClient,
	logger *zap.Logger,
) *OAuthLoginService {
	return &OAuthLoginService{
		cfgProvider:  cfgProvider,
		txm:          txm,
		users:        users,
		roles:        roles,
		binder:       binder,
		states:       states,
		tokens:       tokens,
		loginRecords: loginRecords,
		tasks:        tasks,
		qq:           qq,
		google:       google,
		github:       github,
		linuxdo:      linuxdo,
		x:            x,
		logger:       logger,
	}
}

// StoreOAuthState 保存一次性 OAuth state（供站点配置接口下发授权 URL 时调用）。
func (s *OAuthLoginService) StoreOAuthState(ctx context.Context, provider, state string) error {
	return s.states.Store(ctx, provider, state)
}

// ConsumeOAuthState 校验并消费一次性 OAuth state，防止授权码重放。
//
// state 绑定本次发起的 provider，消费成功后不能再用；它是回调关联校验，不替代上游对授权码的验证。
// 此步骤由接口回调流程调用，Login 方法本身不自动消费 state，新增入口需维持同样的前置校验。
func (s *OAuthLoginService) ConsumeOAuthState(ctx context.Context, provider, state string) error {
	return s.states.Consume(ctx, provider, state)
}

// ProviderEnabled 报告站点是否开启了该第三方登录方式。
func (s *OAuthLoginService) ProviderEnabled(provider string) bool {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Login == nil {
		return false
	}

	switch provider {
	case OAuthProviderQQ:
		return cfg.Site.Login.QQLogin
	case OAuthProviderGoogle:
		return cfg.Site.Login.GoogleLogin
	case OAuthProviderGitHub:
		return cfg.Site.Login.GitHubLogin
	case OAuthProviderLinuxDo:
		return cfg.Site.Login.LinuxDoLogin
	case OAuthProviderX:
		return cfg.Site.Login.XLogin
	default:
		return false
	}
}

// Login 执行第三方登录：换取用户信息 → 查找或创建本地账号 → 签发令牌。
//
// 换取用户信息与查找/创建账号的失败都返回 ErrOAuthProviderFailed，
// 由接口层翻译成「xxx登录失败」；封禁与签发失败按原文案透出。
func (s *OAuthLoginService) Login(ctx context.Context, in OAuthLoginInput) (*jwts.JwtToken, error) {
	info, err := s.fetchUser(ctx, in)
	if err != nil {
		return nil, apperr.Wrap("application/identity/oauth_login_service", ErrOAuthProviderFailed, err)
	}

	user, err := s.findOrCreateUser(ctx, domain.OAuthProfile{
		RegisterSource: in.Provider,
		OpenID:         info.OpenID,
		Email:          info.Email,
		EmailVerified:  info.EmailVerified,
		FallbackEmail:  info.FallbackEmail,
		Avatar:         info.Avatar,
		RegisterIP:     in.IP,
		RegisterAddr:   in.Addr,
	})
	if err != nil {
		return nil, apperr.Wrap("application/identity/oauth_login_service", ErrOAuthProviderFailed, err)
	}

	return s.completeLogin(ctx, user, in)
}

func (s *OAuthLoginService) fetchUser(ctx context.Context, in OAuthLoginInput) (*OAuthUserInfo, error) {
	switch in.Provider {
	case OAuthProviderQQ:
		return s.qq.FetchUser(ctx, in.Code)
	case OAuthProviderGoogle:
		return s.google.FetchUser(ctx, in.Code)
	case OAuthProviderGitHub:
		return s.github.FetchUser(ctx, in.Code, in.CodeVerifier)
	case OAuthProviderLinuxDo:
		return s.linuxdo.FetchUser(ctx, in.Code)
	case OAuthProviderX:
		return s.x.FetchUser(ctx, in.Code, in.CodeVerifier)
	default:
		return nil, fmt.Errorf("%w: %s", ErrOAuthProviderUnsupported, in.Provider)
	}
}

// findOrCreateUser 按 openID+来源、再按邮箱查找既有账号；都没有时创建新账号。
//
// 与原 findOrCreateOAuthUser 的查找顺序、事务边界与 Casbin 补偿逻辑一致。
func (s *OAuthLoginService) findOrCreateUser(ctx context.Context, profile domain.OAuthProfile) (*domain.User, error) {
	profile, err := profile.Normalize()
	if err != nil {
		return nil, err
	}

	email := profile.ResolvedEmail()

	user, err := s.users.FindByOpenIDAndSource(ctx, profile.OpenID, profile.RegisterSource)
	if err != nil {
		return nil, fmt.Errorf("query oauth user failed: %w", err)
	}

	if user != nil {
		return user, nil
	}

	user, err = s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("query oauth user by email failed: %w", err)
	}

	if user != nil {
		// Only an explicit provider assertion can associate a pre-existing email.
		// Synthetic fallback addresses and unverified claims never grant access.
		if !profile.EmailVerified || profile.Email == "" {
			return nil, ErrOAuthAccountBindingRequired
		}

		return user, nil
	}

	created := &domain.User{
		Username:       domain.GenerateUsername(),
		Email:          email,
		Avatar:         profile.Avatar,
		OpenID:         profile.OpenID,
		RegisterIP:     profile.RegisterIP,
		RegisterAddr:   profile.RegisterAddr,
		RegisterSource: profile.RegisterSource,
		Gender:         enum.GenderUnknown,
	}

	// 用户创建钩子需要读取「注册默认横幅 ID」，必须在开启事务前包装 ctx。
	createCtx := s.users.WithDefaultBanner(ctx)

	if err = s.txm.Do(createCtx, func(txCtx context.Context) error {
		roleID, roleErr := s.roles.RoleIDByName(txCtx, constants.CommonUserRole)
		if roleErr != nil {
			return fmt.Errorf("find default role failed: %w", roleErr)
		}

		created.CurrentRoleID = roleID

		if createErr := s.users.Create(txCtx, created, roleID); createErr != nil {
			return fmt.Errorf("create oauth user failed: %w", createErr)
		}

		return s.users.InitializeDefaults(txCtx, created.ID)
	}); err != nil {
		return nil, err
	}

	// Casbin 在 Commit 之后写入，避免 Commit 失败留下幽灵权限。
	if err = s.binder.AddRoleForUser(created.ID, constants.CommonUserRole); err != nil {
		// 补偿删除已提交用户，避免无 Casbin 角色的半成品账号。
		if delErr := s.users.DeleteByID(ctx, created.ID); delErr != nil {
			err = errors.Join(err, delErr)
		}

		return nil, fmt.Errorf("bind oauth casbin role failed: %w", err)
	}

	return created, nil
}

// completeLogin 校验封禁状态、异步记录登录并签发令牌（原 completeOAuthLogin）。
func (s *OAuthLoginService) completeLogin(
	ctx context.Context,
	user *domain.User,
	in OAuthLoginInput,
) (*jwts.JwtToken, error) {
	if user.IsBanned() {
		return nil, domain.ErrAccountBanned
	}

	// 原实现在此处 Association("Roles").Find(&roles) 但从不使用结果，
	// 唯一作用是「角色关联不可读时拒绝登录」。保留该校验以维持错误分支 1:1，
	if _, err := s.roles.LoadRoles(ctx, user.ID); err != nil {
		return nil, apperr.Wrap("application/identity/oauth_login_service", ErrLoginFailed, err)
	}

	recordLoginAsync(
		ctx,
		s.tasks,
		s.loginRecords,
		user.ID,
		in.IP,
		in.Addr,
		in.UserAgent,
		in.Provider,
		s.logger,
	)

	token, err := s.tokens.IssueForUser(ctx, user, s.users)
	if err != nil {
		return nil, apperr.Wrap("application/identity/oauth_login_service", ErrLoginFailed, err)
	}

	return token, nil
}
