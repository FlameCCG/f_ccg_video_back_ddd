package enum

// RegisterSourceType 类型.
type RegisterSourceType string

// RegisterSourceQQType 及相关导出常量.
const (
	// RegisterSourceEmailsType 常量.
	RegisterSourceEmailsType RegisterSourceType = "邮箱注册"
	// RegisterSourceMobileType 常量.
	RegisterSourceMobileType RegisterSourceType = "手机注册"
	// RegisterSourceQQType 常量.
	RegisterSourceQQType      RegisterSourceType = "QQ注册"
	RegisterSourceGoogleType  RegisterSourceType = "Google注册"
	RegisterSourceXType       RegisterSourceType = "X(twitter)注册"
	RegisterSourceGitHubType  RegisterSourceType = "GitHub注册"
	RegisterSourceLinuxDoType RegisterSourceType = "LinuxDo注册"
)
