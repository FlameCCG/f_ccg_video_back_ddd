package config

// Server 配置 HTTP 监听与 Gin 模式，并给 CORS/WebSocket 来源策略提供允许源。
type Server struct {
	Name string `mapstructure:"name" json:"name"`
	Addr string `mapstructure:"addr" json:"addr"`
	Mode string `mapstructure:"mode" json:"mode"`
	// CorsOrigins 允许的浏览器 Origin 白名单（完整 URL 或 host[:port]）。
	// 配置 "*" 表示放开任意 Origin（不带 credentials）。release 建议显式列出前端域名。
	CorsOrigins []string `mapstructure:"corsOrigins" json:"corsOrigins"`
}
