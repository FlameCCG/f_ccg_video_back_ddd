package site

import "math/rand/v2"

// CaptchaOptions 表示当前场景允许使用的交互验证码。
type CaptchaOptions struct {
	Click bool
	Slide bool
}

// InteractiveCaptchaOptions 读取登录或邮箱发送场景的验证码开关。
func (s *Site) InteractiveCaptchaOptions(scene string) CaptchaOptions {
	if s == nil {
		return CaptchaOptions{}
	}
	switch scene {
	case "login":
		if s.Login != nil {
			return CaptchaOptions{Click: s.Login.TextClickCaptcha, Slide: s.Login.SlideCaptcha}
		}
	case "register":
		if s.Register != nil {
			return CaptchaOptions{Click: s.Register.TextClickCaptcha, Slide: s.Register.SlideCaptcha}
		}
	}
	return CaptchaOptions{}
}

// SelectType 在已开启的验证码中等概率选择一种；未开启时返回 none。
func (o CaptchaOptions) SelectType() string {
	if o.Click && o.Slide {
		if rand.IntN(2) == 0 {
			return "click"
		}
		return "slide"
	}
	if o.Click {
		return "click"
	}
	if o.Slide {
		return "slide"
	}
	return "none"
}

// CaptchaSettings 是指定场景、指定类型独立使用的验证码参数。
type CaptchaSettings struct {
	TTLSeconds int
	Padding    int
}

// CaptchaSettings 返回当前场景的参数，不向其他场景回退；边距 0 表示不放宽落点。
func (s *Site) CaptchaSettings(scene, kind string) CaptchaSettings {
	settings := CaptchaSettings{TTLSeconds: 300, Padding: 20}
	if kind == "slide" {
		settings.Padding = 10
	}
	if s == nil {
		return settings
	}
	var ttl, padding int
	switch {
	case scene == "login" && s.Login != nil:
		if kind == "slide" {
			ttl, padding = s.Login.SlideCaptchaTTL, s.Login.SlideCaptchaPadding
		} else {
			ttl, padding = s.Login.TextClickCaptchaTTL, s.Login.TextClickCaptchaPadding
		}
	case scene == "register" && s.Register != nil:
		if kind == "slide" {
			ttl, padding = s.Register.SlideCaptchaTTL, s.Register.SlideCaptchaPadding
		} else {
			ttl, padding = s.Register.TextClickCaptchaTTL, s.Register.TextClickCaptchaPadding
		}
	default:
		return settings
	}
	if ttl > 0 {
		settings.TTLSeconds = ttl
	}
	if padding >= 0 {
		settings.Padding = padding
	}
	return settings
}

// ResolveCaptchaScene 为未指定场景的旧调用提供固定默认值。
func ResolveCaptchaScene(scene, fallback string) string {
	if scene == "" {
		return fallback
	}
	return scene
}
