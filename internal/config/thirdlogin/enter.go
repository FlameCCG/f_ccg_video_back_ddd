// Package third_login 定义 QQ、Google、GitHub、Linux.do 和 X 的 OAuth 客户端配置。
//
// 这些结构体只携带客户端标识、密钥和回调地址，不发送授权请求或保存 OAuth state。
// 授权 URL 由运营用例组织，登录用例校验 state 并通过 infrastructure/external/oauth 调用上游。
// 回调地址需要与第三方平台登记值一致；字段变更还要核对 YAML 与管理端配置序列化。
package third_login

// QQ 描述 QQ OAuth 客户端的应用标识、密钥与回调 URL；启用开关位于站点登录配置。
type QQ struct {
	AppID    string `mapstructure:"appID" json:"appID"`
	AppKey   string `mapstructure:"appKey" json:"appKey"`
	Redirect string `mapstructure:"redirect" json:"redirect"`
}

// Google 描述 Google OAuth 客户端的应用标识、密钥与回调 URL；启用开关位于站点登录配置。
type Google struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// GitHub 描述 GitHub OAuth 客户端的应用标识、密钥与回调 URL；启用开关位于站点登录配置。
type GitHub struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// LinuxDo 描述 Linux.do OAuth 客户端的应用标识、密钥与回调 URL；启用开关位于站点登录配置。
type LinuxDo struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// X 描述 X OAuth 客户端的应用标识、密钥与回调 URL；启用开关位于站点登录配置。
type X struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}
