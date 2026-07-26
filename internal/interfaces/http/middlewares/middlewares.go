// Package middlewares 提供 gin 中间件（原 pkg/middlewares）。
//
// 原实现全部为包级函数并直读 global.*；新架构把有依赖的中间件收敛为
// *Middlewares 的方法，依赖由 wire 注入；无状态的泛型绑定与工具函数仍为包级函数。
package middlewares

import (
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/cache/ratelimit"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/casbin/casbin/v2"
)

// Middlewares 聚合所有需要依赖注入的 gin 中间件。
type Middlewares struct {
	cfgProvider *config.Provider
	enforcer    *casbin.SyncedEnforcer
	userStatus  *identityapp.UserStatusService
	bans        *identityapp.BanService
	banRecords  *identityapp.BanRecordService
	captcha     *identityapp.CaptchaService
	limiter     *ratelimit.Limiter
}

// New 构造中间件集合。
func New(
	cfgProvider *config.Provider,
	enforcer *casbin.SyncedEnforcer,
	userStatus *identityapp.UserStatusService,
	bans *identityapp.BanService,
	banRecords *identityapp.BanRecordService,
	captcha *identityapp.CaptchaService,
	limiter *ratelimit.Limiter,
) *Middlewares {
	return &Middlewares{
		cfgProvider: cfgProvider,
		enforcer:    enforcer,
		userStatus:  userStatus,
		bans:        bans,
		banRecords:  banRecords,
		captcha:     captcha,
		limiter:     limiter,
	}
}
