package config

import "time"

// Email 类型.
type Email struct {
	Domain       string        `mapstructure:"domain" json:"domain"`
	Port         int           `mapstructure:"port" json:"port"`
	SendEmail    string        `mapstructure:"sendEmail" json:"sendEmail"`
	AuthCode     string        `mapstructure:"authCode" json:"authCode"`
	SendNickName string        `mapstructure:"sendNickName" json:"sendNickName"`
	SSL          bool          `mapstructure:"ssl" json:"ssl"`
	TLS          bool          `mapstructure:"tls" json:"tls"`
	IsExpire     time.Duration `mapstructure:"isExpire" json:"isExpire"`
}
