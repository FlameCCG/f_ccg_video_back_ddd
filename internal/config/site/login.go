package site

// Login 配置登录方式与交互验证码候选项，验证码 TTL 和 Padding 分别影响有效期与交互区域。
// 多个交互验证码启用时由场景选择逻辑决定本次类型，不表示用户必须逐一完成所有候选。
type Login struct {
	QQLogin                 bool `mapstructure:"qqLogin" json:"qqLogin"`
	XLogin                  bool `mapstructure:"xLogin" json:"xLogin"`
	GoogleLogin             bool `mapstructure:"googleLogin" json:"googleLogin"`
	GitHubLogin             bool `mapstructure:"gitHubLogin" json:"gitHubLogin"`
	LinuxDoLogin            bool `mapstructure:"linuxdoLogin" json:"linuxdoLogin"`
	UsernamePwdLogin        bool `mapstructure:"usernamePwdLogin" json:"usernamePwdLogin"`
	TextGraphicCaptcha      bool `mapstructure:"textGraphicCaptcha" json:"textGraphicCaptcha"`
	TextClickCaptcha        bool `mapstructure:"textClickCaptcha" json:"textClickCaptcha"`
	SlideCaptcha            bool `mapstructure:"slideCaptcha" json:"slideCaptcha"`
	SlideCaptchaTTL         int  `mapstructure:"slideCaptchaTTL" json:"slideCaptchaTTL"`
	SlideCaptchaPadding     int  `mapstructure:"slideCaptchaPadding" json:"slideCaptchaPadding"`
	TextClickCaptchaTTL     int  `mapstructure:"textClickCaptchaTTL" json:"textClickCaptchaTTL"`
	TextClickCaptchaPadding int  `mapstructure:"textClickCaptchaPadding" json:"textClickCaptchaPadding"`
}
