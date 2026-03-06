package config

// Redis 描述共享 Redis 连接的地址、逻辑库和池大小；具体 key 与 TTL 由上下文适配器定义。
type Redis struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     string `mapstructure:"port" json:"port"`
	Password string `mapstructure:"password" json:"password"`
	Db       int    `mapstructure:"db" json:"db"`
	PoolSize int    `mapstructure:"poolSize" json:"poolSize"`
}
