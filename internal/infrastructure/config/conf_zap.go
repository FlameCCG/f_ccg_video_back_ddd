package config

// Logger 类型.
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
