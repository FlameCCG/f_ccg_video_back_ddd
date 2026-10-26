package config

// Jwt 配置访问/刷新令牌的独立密钥与有效天数。
// 过期字段存储天数，应用层校验范围后转换为 time.Duration。
type Jwt struct {
	AccessExp     int64  `mapstructure:"accessExp" json:"accessExp"`   // 天
	RefreshExp    int64  `mapstructure:"refreshExp" json:"refreshExp"` // 天
	AccessSecret  string `mapstructure:"accessSecret" json:"accessSecret"`
	RefreshSecret string `mapstructure:"refreshSecret" json:"refreshSecret"`
}
