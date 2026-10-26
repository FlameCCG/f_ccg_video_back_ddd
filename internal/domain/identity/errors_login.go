package identity

import "errors"

// 注册/登录/OAuth/资料相关的领域错误。
//
// 文案与原 api/user_api 各处 res.FailWithMsg 的字面量逐字一致：
// 应用层直接返回这些哨兵错误，接口层用 err.Error() 输出，保证响应文案 1:1。
var (
	// ErrUserOrEmailExists 注册时用户名或邮箱已存在。
	ErrUserOrEmailExists = errors.New("用户名或邮箱已存在")
	// ErrPasswordHashFailed bcrypt 加密失败。
	ErrPasswordHashFailed = errors.New("密码加密失败")
	// ErrCredentialsInvalid 用户名或密码错误。
	ErrCredentialsInvalid = errors.New("用户名或密码错误")
	// ErrAccountPermanentlyBanned 账号已被永久封禁。
	ErrAccountPermanentlyBanned = errors.New("账号已被永久封禁")
	// ErrOldPasswordRequired 已设密用户修改密码必须提供旧密码。
	ErrOldPasswordRequired = errors.New("请输入旧密码")
	// ErrOldPasswordIncorrect 旧密码校验失败。
	ErrOldPasswordIncorrect = errors.New("旧密码错误")
	// ErrEmailNotRegistered 重置密码时邮箱未注册。
	ErrEmailNotRegistered = errors.New("邮箱未注册")
	// ErrEmailOccupied 绑定邮箱时邮箱已被其它账号占用。
	ErrEmailOccupied = errors.New("邮箱已被占用")
	// ErrUsernameExists 更新资料时用户名已被占用。
	ErrUsernameExists = errors.New("用户名已存在")
	// ErrGenderInvalid 性别取值非法。
	ErrGenderInvalid = errors.New("性别参数错误")
	// ErrBirthdayFormatInvalid 生日格式非 2006-01-02。
	ErrBirthdayFormatInvalid = errors.New("生日格式错误")
	// ErrNoFieldsToUpdate 请求未携带任何可更新字段。
	ErrNoFieldsToUpdate = errors.New("没有需要更新的字段")
	// ErrUserConfNotFound 用户配置不存在。
	ErrUserConfNotFound = errors.New("用户配置不存在")
	// ErrRoleNotOwned 用户未拥有目标角色。
	ErrRoleNotOwned = errors.New("用户未拥有该角色")
	// ErrRecentLikedNotPublic 目标用户未公开最近点赞。
	ErrRecentLikedNotPublic = errors.New("用户未公开最近点赞")
	// ErrRecentCoinedNotPublic 目标用户未公开最近投币。
	ErrRecentCoinedNotPublic = errors.New("用户未公开最近投币")

	// ErrOAuthStateInvalid OAuth state 缺失、已消费或已过期。
	ErrOAuthStateInvalid = errors.New("oauth state is invalid")
	// ErrOAuthRegisterSourceEmpty OAuth 注册来源为空。
	ErrOAuthRegisterSourceEmpty = errors.New("oauth register source is empty")
	// ErrOAuthOpenIDEmpty OAuth openID 为空。
	ErrOAuthOpenIDEmpty = errors.New("oauth open id is empty")

	// ErrUserBannerNotFound 用户主页横幅不存在（非用户类型或未展示）。
	ErrUserBannerNotFound = errors.New("用户主页横幅不存在")
	// ErrSystemDefaultBannerNotFound 系统默认横幅不存在。
	ErrSystemDefaultBannerNotFound = errors.New("系统默认横幅不存在")
	// ErrBannerIDRequired bannerId 不能为空。
	ErrBannerIDRequired = errors.New("bannerId不能为空")
	// ErrBannerIDsRequired bannerIds 不能为空。
	ErrBannerIDsRequired = errors.New("bannerIds不能为空")
	// ErrRegisterDefaultBannerLocked 注册默认横幅不允许在默认列表中增删。
	ErrRegisterDefaultBannerLocked = errors.New("默认注册横幅不允许操作")
	// ErrBannerAlreadyInDefaults 所选横幅已存在于系统默认列表中。
	ErrBannerAlreadyInDefaults = errors.New("所选横幅已存在于系统默认列表中")
	// ErrBannerActionInvalid 横幅列表操作类型非法。
	ErrBannerActionInvalid = errors.New("操作类型错误")
	// ErrBannerAlreadyRegisterDefault 该横幅已是注册默认横幅。
	ErrBannerAlreadyRegisterDefault = errors.New("该横幅已是注册默认横幅")
)

// ErrAccountBanned indicates that the account is currently banned.
var ErrAccountBanned = errors.New("account is banned")
