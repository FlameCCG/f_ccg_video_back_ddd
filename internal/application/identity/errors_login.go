package identity

import "errors"

// 注册/登录/资料/横幅/邮箱验证码用例的错误。
//
// 文案与原 api/user_api、api/captcha_api 各处 res.FailWithMsg 的字面量逐字一致：
// 接口层统一用 err.Error() 输出，保证响应文案 1:1。
var (
	// ErrRegisterFailed 注册流程失败（角色缺失/落库失败/事务提交失败/Casbin 绑定失败）。
	ErrRegisterFailed = errors.New("注册失败")
	// ErrPwdLoginDisabled 站点关闭了用户名密码登录。
	ErrPwdLoginDisabled = errors.New("暂不支持用户名密码登录")
	// ErrQueryUserFailed 查询用户时数据库出错。
	ErrQueryUserFailed = errors.New("查询用户失败")
	// ErrLoginFailed 登录收尾失败（角色加载失败 / JWT 签发失败）。
	ErrLoginFailed = errors.New("登录失败")
	// ErrAdminRoleMissing 管理员角色不存在。
	ErrAdminRoleMissing = errors.New("管理员角色不存在")
	// ErrRoleCheckFailed 角色校验查询失败。
	ErrRoleCheckFailed = errors.New("角色校验失败")
	// ErrNotAdminAccount 非管理员账号尝试登录后台。
	ErrNotAdminAccount = errors.New("非管理员账号")
	// ErrRefreshTokenInvalid refreshToken 无效。
	ErrRefreshTokenInvalid = errors.New("refreshToken无效")
	// ErrRefreshTokenFailed 刷新令牌签发失败。
	ErrRefreshTokenFailed = errors.New("刷新令牌失败")

	// ErrOAuthProviderFailed 第三方登录失败（换取用户信息或查找/创建账号失败）。
	// 接口层据此翻译为「qq登录失败」「github登录失败」等 provider 专属文案。
	ErrOAuthProviderFailed = errors.New("oauth provider login failed")
	// ErrOAuthProviderUnsupported 未支持的第三方登录提供商。
	ErrOAuthProviderUnsupported = errors.New("unsupported oauth provider")

	// ErrChangePasswordFailed 修改密码落库失败。
	ErrChangePasswordFailed = errors.New("修改失败")
	// ErrResetPasswordFailed 重置密码失败。
	ErrResetPasswordFailed = errors.New("重置失败")
	// ErrBindEmailFailed 绑定邮箱落库失败。
	ErrBindEmailFailed = errors.New("绑定失败")
	// ErrSwitchRoleFailed 切换角色落库失败。
	ErrSwitchRoleFailed = errors.New("切换失败")
	// ErrUpdateUserFailed 更新用户资料/配置失败。
	ErrUpdateUserFailed = errors.New("更新失败")

	// ErrBannerListFailed 获取系统默认横幅列表失败。
	ErrBannerListFailed = errors.New("获取失败")
	// ErrPersistSiteConfigFailed 站点配置写回文件失败。
	ErrPersistSiteConfigFailed = errors.New("更新配置文件失败")
	// ErrSiteConfigNotInitialized 站点配置未初始化。
	ErrSiteConfigNotInitialized = errors.New("站点配置未初始化")
	// ErrSetUserBannerFailed 批量迁移用户横幅失败。
	ErrSetUserBannerFailed = errors.New("设置失败")
	// ErrStorageConfigInvalid 存储驱动配置错误。
	ErrStorageConfigInvalid = errors.New("存储配置错误")
	// ErrBannerUploadFailed 横幅文件保存失败。
	ErrBannerUploadFailed = errors.New("横幅上传失败")

	// ErrEmailCaptchaRateLimited 邮箱验证码发送被限流。
	ErrEmailCaptchaRateLimited = errors.New("验证码发送过于频繁，请稍后再试")
	// ErrEmailVerifyFailed 邮箱可达性校验调用失败。
	ErrEmailVerifyFailed = errors.New("邮箱验证失败")
	// ErrEmailSyntaxInvalid 邮箱语法错误。
	ErrEmailSyntaxInvalid = errors.New("邮箱格式错误")
	// ErrEmailHostNotExists 邮箱服务器不存在。
	ErrEmailHostNotExists = errors.New("邮箱服务器不存在")
	// ErrEmailNoMXRecords 邮箱域名无 MX 记录。
	ErrEmailNoMXRecords = errors.New("邮箱服务器无MX记录")
	// ErrEmailAlreadyRegistered 注册验证码：邮箱已注册。
	ErrEmailAlreadyRegistered = errors.New("该邮箱已注册")
	// ErrEmailCaptchaEmailNotRegistered 重置密码验证码：邮箱未注册。
	ErrEmailCaptchaEmailNotRegistered = errors.New("该邮箱未注册")
	// ErrEmailAlreadyBound 绑定邮箱验证码：邮箱已被绑定。
	ErrEmailAlreadyBound = errors.New("该邮箱已被绑定")
	// ErrEmailCaptchaStoreFailed 验证码写入存储失败。
	ErrEmailCaptchaStoreFailed = errors.New("验证码保存失败")
)

var ErrOAuthAccountBindingRequired = errors.New("existing account requires explicit OAuth binding")

var ErrRefreshTokenReused = errors.New("refresh token has already been used")

var ErrInitialPasswordUnchanged = errors.New("new password must differ from initial password")
