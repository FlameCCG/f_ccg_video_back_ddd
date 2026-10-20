package site

// Register 类型.
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
