package config

// Email 描述 SMTP 发信身份、连接方式与验证码有效期。
// SSL/TLS 的握手分支和 IsExpire 的换算需结合 external/email 与 identity 验证码用例阅读。
type Email struct {
	Domain       string `mapstructure:"domain" json:"domain"`
	Port         int    `mapstructure:"port" json:"port"`
	SendEmail    string `mapstructure:"sendEmail" json:"sendEmail"`
	AuthCode     string `mapstructure:"authCode" json:"authCode"`
	SendNickName string `mapstructure:"sendNickName" json:"sendNickName"`
	SSL          bool   `mapstructure:"ssl" json:"ssl"`
	TLS          bool   `mapstructure:"tls" json:"tls"`
	IsExpire     int64  `mapstructure:"isExpire" json:"isExpire"`
}
