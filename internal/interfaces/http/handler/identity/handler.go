// Package identity 是身份上下文的 HTTP 处理器（原 api/user_api 与 api/captcha_api）。
//
// 方法名与原 UserAPI / CaptchaAPI 的导出方法完全一致；请求/响应结构体的字段与 tag 逐字照搬，
// 保证 JSON 契约 1:1。处理器只做请求解析、参数装配与响应文案映射，业务逻辑全部在 application 层。
// gin.Context 止步于本包：向应用层只传 context.Context 与 DTO。
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
	logger       *zap.Logger
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
	logger *zap.Logger,
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
		logger:       loggerOrNop(logger),
	}
}

func (h *Handler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}
