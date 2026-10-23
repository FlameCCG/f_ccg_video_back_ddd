package site

// Register 配置注册场景的验证码能力；邮件验证与交互验证码是分别控制的步骤。
type Register struct {
	EmailCaptcha            bool `mapstructure:"emailCaptcha" json:"emailCaptcha"`
	TextGraphicCaptcha      bool `mapstructure:"textGraphicCaptcha" json:"textGraphicCaptcha"`
	SlideCaptcha            bool `mapstructure:"slideCaptcha" json:"slideCaptcha"`
	TextClickCaptcha        bool `mapstructure:"textClickCaptcha" json:"textClickCaptcha"`
	TextClickCaptchaTTL     int  `mapstructure:"textClickCaptchaTTL" json:"textClickCaptchaTTL"`
	TextClickCaptchaPadding int  `mapstructure:"textClickCaptchaPadding" json:"textClickCaptchaPadding"`
	SlideCaptchaTTL         int  `mapstructure:"slideCaptchaTTL" json:"slideCaptchaTTL"`
	SlideCaptchaPadding     int  `mapstructure:"slideCaptchaPadding" json:"slideCaptchaPadding"`
}
