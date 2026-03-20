package identity

// OAuthLoginInput 第三方登录入参。
type OAuthLoginInput struct {
	// Provider 第三方标识（qq/google/github/linuxdo/x），同时作为 register_source。
	Provider string
	// Code 授权码。
	Code string
	// CodeVerifier PKCE 校验串（github 可选、x 必填）。
	CodeVerifier string
	// IP 客户端 IP。
	IP string
	// Addr 客户端 IP 归属地。
	Addr string
	// UserAgent 请求 UA，用于登录记录。
	UserAgent string
}
