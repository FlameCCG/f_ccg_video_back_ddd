package enum

// RegisterSourceType 标识账号注册/第三方身份来源；与 open_id 一起参与身份唯一性判断。
type RegisterSourceType string

const (
	// RegisterSourceEmailsType 表示邮箱注册。
	RegisterSourceEmailsType RegisterSourceType = "邮箱注册"
	// RegisterSourceMobileType 保留手机注册的来源值。
	RegisterSourceMobileType RegisterSourceType = "手机注册"
	// RegisterSourceQQType 表示 QQ 注册或第三方建号来源。
	RegisterSourceQQType      RegisterSourceType = "QQ注册"
	RegisterSourceGoogleType  RegisterSourceType = "Google注册"
	RegisterSourceXType       RegisterSourceType = "X(twitter)注册"
	RegisterSourceGitHubType  RegisterSourceType = "GitHub注册"
	RegisterSourceLinuxDoType RegisterSourceType = "LinuxDo注册"
)
