// Package identity 提供账号与验证码 HTTP 处理器。
//
// 请求与响应 JSON 字段遵循接口契约；处理器解析参数并映射响应，业务逻辑由应用层处理。
// gin.Context 止于接口层，应用层仅接收 context.Context 与 DTO。
package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"go.uber.org/zap"
)

// Handler 提供 /v1/common/user 用户侧 HTTP 接口。
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

// NewHandler 构造用户侧处理器，resolver 用于解析客户端 IP 归属地。
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
