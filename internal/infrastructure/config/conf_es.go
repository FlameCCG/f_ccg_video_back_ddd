package config

// ES Elasticsearch配置.
type ES struct {
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	IsHTTPS  bool   `mapstructure:"https" json:"https"`
	// InsecureSkipVerify 仅允许开发环境连接自签名 ES 时开启。
	InsecureSkipVerify bool `mapstructure:"insecureSkipVerify" json:"insecureSkipVerify"`
}
