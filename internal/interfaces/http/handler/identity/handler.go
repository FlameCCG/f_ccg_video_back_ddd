// Package identity 把账号登录、注册、资料、验证码与第三方回调的 HTTP 请求翻译为应用用例调用。
//
// handler.go 声明显式依赖，其他文件按入口拆分；参数结构体保留 Gin 绑定与校验标签。
// 从请求取得参数和经过鉴权的身份，构造应用参数后传 c.Request.Context()，最后映射错误和响应。
// gin.Context 止步于接口层，领域规则、事务、缓存与跨上下文调用由 application 编排。
// 登录场景的验证码与限流由路由中间件和应用用例共同约束；处理器不能自行认定请求中的身份可信。
// JSON 字段、状态码和响应文案属于对外契约，新增注释不应顺带调整这些行为。
package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"go.uber.org/zap"
)

// Handler 用户侧（/v1/common/user）HTTP 处理器。
//
// 对应原 api/user_api.UserAPI 中挂在 common 分组下的方法。
type Handler struct {
	activity     *app.UserActivityService
	profiles     *app.ProfileService
	confs        *app.UserConfService
	banners      *app.UserBannerService
	accounts     *app.AccountService
	registration *app.RegistrationService
	logins       *app.LoginService
	oauth        *app.OAuthLoginService
	creators     *app.CreatorAnalyticsService
	loginRecords *app.LoginRecordService
	expRecords   *app.ExpRecordService
	coinRecords  *app.CoinRecordService
	resolver     ipUtils.RegionResolver
}

// NewHandler 构造用户侧处理器。
//
// resolver 用于把客户端 IP 解析为归属地（原 ipUtils 直读 global.IPSearcher，见 decisions.md D-005）。
func NewHandler(
	activity *app.UserActivityService,
	profiles *app.ProfileService,
	confs *app.UserConfService,
	banners *app.UserBannerService,
	accounts *app.AccountService,
	registration *app.RegistrationService,
	logins *app.LoginService,
	oauth *app.OAuthLoginService,
	creators *app.CreatorAnalyticsService,
	loginRecords *app.LoginRecordService,
	expRecords *app.ExpRecordService,
	coinRecords *app.CoinRecordService,
	resolver ipUtils.RegionResolver,
	_ *zap.Logger,
) *Handler {
	return &Handler{
		activity:     activity,
		profiles:     profiles,
		confs:        confs,
		banners:      banners,
		accounts:     accounts,
		registration: registration,
		logins:       logins,
		oauth:        oauth,
		creators:     creators,
		loginRecords: loginRecords,
		expRecords:   expRecords,
		coinRecords:  coinRecords,
		resolver:     resolver,
	}
}
