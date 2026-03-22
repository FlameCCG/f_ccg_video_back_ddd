package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"f_ccg_video_back_ddd/internal/application/shared"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"
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

// OAuthLoginService 第三方登录用例服务。
type OAuthLoginService struct {
	cfgProvider  shared.ConfigReader
	txm          tx.Manager
	users        identitydomain.UserAccountRepository
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
	usernames    identitydomain.UsernamePolicy
	folders      DefaultFolderInitializer
}

// NewOAuthLoginService 构造第三方登录服务。
func NewOAuthLoginService(
	cfgProvider shared.ConfigReader,
	txm tx.Manager,
	users identitydomain.UserAccountRepository,
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
	folders DefaultFolderInitializer,
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
		usernames:    identitydomain.UsernamePolicy{},
		folders:      folders,
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

// Login 执行第三方登录：换取用户信息、查找或创建本地账号、签发令牌。
//
// 用户信息获取与账号查找/创建失败返回 ErrOAuthProviderFailed，由接口层转换为登录失败文案；封禁与签发错误直接返回对应文案。
func (s *OAuthLoginService) Login(ctx context.Context, in OAuthLoginInput) (*jwts.JwtToken, error) {
	label := s.logLabel(in.Provider)

	info, err := s.fetchUser(ctx, in)
	if err != nil {
		s.logger.Error(s.logLabel(in.Provider)+" Login Error", zap.Error(err))

		return nil, ErrOAuthProviderFailed
	}

	user, err := s.findOrCreateUser(ctx, identitydomain.OAuthProfile{
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
		s.logger.Error(label+" Login Find Or Create User Failed", zap.Error(err))

		return nil, ErrOAuthProviderFailed
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

// findOrCreateUser 按 openID+来源、再按邮箱查找账号；均不存在时创建新账号。
//
// 负责 OAuth 用户查找、创建、事务写入与 Casbin 权限补偿。
func (s *OAuthLoginService) findOrCreateUser(ctx context.Context, profile identitydomain.OAuthProfile) (*identitydomain.User, error) {
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
		if !profile.CanClaimExistingEmail() {
			return nil, ErrOAuthAccountBindingRequired
		}
		return user, nil
	}

	username := s.usernames.Generate(uuid.New().String(), time.Now())
	var created *identitydomain.User

	if err = s.txm.Do(ctx, func(txCtx context.Context) error {
		roleID, roleErr := s.roles.RoleIDByName(txCtx, constants.CommonUserRole)
		if roleErr != nil {
			return fmt.Errorf("find default role failed: %w", roleErr)
		}

		created = identitydomain.NewOAuthUser(username, email, profile.Avatar, profile.OpenID, profile.RegisterIP, profile.RegisterAddr, profile.RegisterSource, roleID)

		saved, createErr := s.users.Create(txCtx, created, roleID)
		if createErr != nil {
			return fmt.Errorf("create oauth user failed: %w", createErr)
		}

		created = saved

		if err := s.users.InitializeDefaults(txCtx, created.ID()); err != nil {
			return err
		}
		return s.folders.InitializeDefaultFolder(txCtx, created.ID())
	}); err != nil {
		return nil, err
	}

	// Casbin 在 Commit 之后写入，避免 Commit 失败留下幽灵权限。
	if err = s.binder.AddRoleForUser(created.ID(), constants.CommonUserRole); err != nil {
		// 补偿删除已提交用户，避免无 Casbin 角色的半成品账号。
		if delErr := s.users.DeleteByID(ctx, created.ID()); delErr != nil {
			s.logger.Error("compensate delete oauth user failed",
				zap.Error(delErr),
				zap.Uint("userId", created.ID()),
				zap.NamedError("casbinErr", err),
			)
		}

		return nil, fmt.Errorf("bind oauth casbin role failed: %w", err)
	}

	return created, nil
}

// completeLogin 校验封禁状态、异步记录登录并签发令牌。
func (s *OAuthLoginService) completeLogin(
	ctx context.Context,
	user *identitydomain.User,
	in OAuthLoginInput,
) (*jwts.JwtToken, error) {
	if err := user.EnsureCanLogin(); err != nil {
		return nil, err
	}

	// 角色关联不可读时拒绝登录。
	if _, err := s.roles.LoadRoles(ctx, user.ID()); err != nil {
		s.logger.Error("获取用户角色失败", zap.Error(err))

		return nil, ErrLoginFailed
	}

	(asyncLoginLog{tasks: s.tasks, records: s.loginRecords, logger: s.logger}).write(
		ctx,
		user.ID(),
		in.IP,
		in.Addr,
		in.UserAgent,
		in.Provider,
	)

	token, err := s.tokens.IssueForUser(ctx, user, s.users)
	if err != nil {
		s.logger.Error("create jwt token failed", zap.Error(err))

		return nil, ErrLoginFailed
	}

	return token, nil
}

// logLabel 返回日志中使用的提供商显示名。
func (s *OAuthLoginService) logLabel(provider string) string {
	switch provider {
	case OAuthProviderQQ:
		return "QQ"
	case OAuthProviderGoogle:
		return "Google"
	case OAuthProviderGitHub:
		return "GitHub"
	case OAuthProviderLinuxDo:
		return "LinuxDo"
	case OAuthProviderX:
		return "X"
	default:
		return provider
	}
}
