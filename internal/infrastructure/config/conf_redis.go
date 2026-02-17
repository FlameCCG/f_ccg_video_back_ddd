package config

// Redis 类型.
type Redis struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     string `mapstructure:"port" json:"port"`
	Password string `mapstructure:"password" json:"password"`
	Db       int    `mapstructure:"db" json:"db"`
	PoolSize int    `mapstructure:"poolSize" json:"poolSize"`
}
