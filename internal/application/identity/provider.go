package identity

import "github.com/google/wire"

// ProviderSet 汇总身份上下文的应用服务构造器。
var ProviderSet = wire.NewSet(
	NewAccountService,
	NewAdminUserService,
	NewBanRecordService,
	NewBanService,
	NewCaptchaService,
	NewCoinRecordService,
	NewCoinService,
	NewCreatorAnalyticsService,
	NewEmailCaptchaSendService,
	NewExpRecordService,
	NewExpService,
	NewLoginRecordService,
	NewLoginService,
	NewOAuthLoginService,
	NewProfileService,
	NewRegistrationService,
	NewTokenService,
	NewUserActivityService,
	NewUserBannerService,
	NewUserConfService,
	NewUserStatusService,
)
