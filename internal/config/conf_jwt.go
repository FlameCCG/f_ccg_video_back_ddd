package config

import "time"

// Jwt 配置访问/刷新令牌的独立密钥与有效天数。
// 虽然过期字段类型为 time.Duration，当前应用层按“天”换算后才交给 JWT 工具；不要直接当纳秒使用。
type Jwt struct {
	AccessExp     time.Duration `mapstructure:"accessExp" json:"accessExp"`   // 天
	RefreshExp    time.Duration `mapstructure:"refreshExp" json:"refreshExp"` // 天
	AccessSecret  string        `mapstructure:"accessSecret" json:"accessSecret"`
	RefreshSecret string        `mapstructure:"refreshSecret" json:"refreshSecret"`
}
