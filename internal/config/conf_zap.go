package config

// Logger 描述各级别日志文件、控制台输出与轮转保留策略。
// 字段只提供构造参数，实际编码与输出行为在 infrastructure/logger 中实现。
type Logger struct {
	DebugFileName string `mapstructure:"debugFileName" json:"debugFileName"`
	InfoFileName  string `mapstructure:"infoFileName" json:"infoFileName"`
	WarnFileName  string `mapstructure:"warnFileName" json:"warnFileName"`
	ErrorFileName string `mapstructure:"errorFileName" json:"errorFileName"`
	EnableConsole bool   `mapstructure:"enableConsole" json:"enableConsole"`
	MaxSize       int    `mapstructure:"maxSize" json:"maxSize"`
	MaxAge        int    `mapstructure:"maxAge" json:"maxAge"`
	MaxBackups    int    `mapstructure:"maxBackups" json:"maxBackups"`
	Development   bool   `mapstructure:"development" json:"development"`
}
