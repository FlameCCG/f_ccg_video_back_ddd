package config

import "time"

// Jwt 类型.
type Jwt struct {
	AccessExp     time.Duration `mapstructure:"accessExp" json:"accessExp"`   // 天
	RefreshExp    time.Duration `mapstructure:"refreshExp" json:"refreshExp"` // 天
	AccessSecret  string        `mapstructure:"accessSecret" json:"accessSecret"`
	RefreshSecret string        `mapstructure:"refreshSecret" json:"refreshSecret"`
}
