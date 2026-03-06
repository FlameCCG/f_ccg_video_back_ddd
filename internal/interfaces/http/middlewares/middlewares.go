// Package middlewares 实现 HTTP 请求进入用例前的通用协议与身份处理。
//
// Middlewares 的依赖由组合根注入；鉴权、封禁、投稿开关、验证码、限流和超时围绕 Gin 请求执行。
// 中间件顺序会影响可用的身份字段与失败响应，例如读取 UID 前需要先建立可信认证上下文。
// params_bind.go 处理参数绑定，uid.go 集中提取用户 ID，业务层只接收普通值与 context.Context。
// JWT 证明令牌通过验证，Casbin 决定当前路由权限，领域仍校验作者身份和资源状态。
// RequestTimeout 传播取消信号，耗时外部调用还需实际使用 context；它不等同于强制终止任意 Go 代码。
package middlewares

import (
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"
)

// Middlewares 聚合所有需要依赖注入的 gin 中间件。
type Middlewares struct {
	cfgProvider *infraconfig.Provider
	enforcer    *casbin.SyncedEnforcer
	userStatus  *identityapp.UserStatusService
	userRoles   identityapp.UserRoleQuery
	bans        *identityapp.BanService
	banRecords  *identityapp.BanRecordService
	captcha     *identityapp.CaptchaService
	limiter     *ratelimit.Limiter
	logger      *zap.Logger
}

// New 构造中间件集合。
func New(
	cfgProvider *infraconfig.Provider,
	enforcer *casbin.SyncedEnforcer,
	userStatus *identityapp.UserStatusService,
	userRoles identityapp.UserRoleQuery,
	bans *identityapp.BanService,
	banRecords *identityapp.BanRecordService,
	captcha *identityapp.CaptchaService,
	limiter *ratelimit.Limiter,
	logger *zap.Logger,
) *Middlewares {
	return &Middlewares{
		cfgProvider: cfgProvider,
		enforcer:    enforcer,
		userStatus:  userStatus,
		userRoles:   userRoles,
		bans:        bans,
		banRecords:  banRecords,
		captcha:     captcha,
		limiter:     limiter,
		logger:      loggerOrNop(logger),
	}
}

func (m *Middlewares) log() *zap.Logger {
	if m == nil {
		return zap.NewNop()
	}

	return loggerOrNop(m.logger)
}
