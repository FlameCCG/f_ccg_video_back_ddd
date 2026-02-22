package site

// Login 类型.
type Login struct {
	QQLogin                 bool `mapstructure:"qqLogin" json:"qqLogin"`
	XLogin                  bool `mapstructure:"xLogin" json:"xLogin"`
	GoogleLogin             bool `mapstructure:"googleLogin" json:"googleLogin"`
	GitHubLogin             bool `mapstructure:"gitHubLogin" json:"gitHubLogin"`
	LinuxDoLogin            bool `mapstructure:"linuxdoLogin" json:"linuxdoLogin"`
	UsernamePwdLogin        bool `mapstructure:"usernamePwdLogin" json:"usernamePwdLogin"`
	TextGraphicCaptcha      bool `mapstructure:"textGraphicCaptcha" json:"textGraphicCaptcha"`
	TextClickCaptcha        bool `mapstructure:"textClickCaptcha" json:"textClickCaptcha"`
	TextClickCaptchaTTL     int  `mapstructure:"textClickCaptchaTTL" json:"textClickCaptchaTTL"`
	TextClickCaptchaPadding int  `mapstructure:"textClickCaptchaPadding" json:"textClickCaptchaPadding"`
}
