package email

import (
	"fmt"
)

// 验证码邮件模板中的固定素材地址（与原 service/email_service/enter.go 完全一致）。
const (
	// registerBgImageURL 注册/重置密码邮件背景图。
	registerBgImageURL = "https://i0.hdslb.com/bfs/feed-admin/e7c2e3678566a707168051a6603a11c1d8557ee0.png@180w_180h_1c_!web-avatar-nav.avif"
	// logoIconURL 顶部 Logo 图标。
	logoIconURL = "https://www.bilibili.com/favicon.ico"
	// registerCharacterImageURL 注册/重置密码邮件角色图。
	registerCharacterImageURL = "https://i0.hdslb.com/bfs/face/e8f775084992569503463a5628b031b23901b7a2.jpg@80w_80h_1c_100q.webp"
	// bindBgImageURL 绑定邮箱邮件背景图。
	bindBgImageURL = "https://i.pximg.net/img-original/img/2025/10/14/20/36/01/136274117_p0.png"
	// bindCharacterImageURL 绑定邮箱邮件角色图。
	bindCharacterImageURL = "https://i.pximg.net/img-original/img/2025/10/19/20/10/18/136467615_p0.png"

	// captchaNoteText 验证码下方提示文案。
	captchaNoteText = "请在10分钟内使用，逾期作废。切勿将验证码告知他人哦！"
)

// 邮箱验证码类型（与接口层 EmailCaptchaRequest.Type 的取值一致）。
const (
	// CaptchaTypeRegister 注册。
	CaptchaTypeRegister = 1
	// CaptchaTypeResetPassword 重置密码。
	CaptchaTypeResetPassword = 2
	// CaptchaTypeBindEmail 绑定邮箱。
	CaptchaTypeBindEmail = 3
)

// SendEmailCaptcha 按验证码类型投递验证码邮件。
//
// 实现 application/identity.EmailCaptchaSender 端口，
// 取代原 api/captcha_api 的包级函数变量 emailCaptchaSend（该变量原本只为测试打桩）。
// 错误文案与原 sendEmailCaptcha 的 default 分支一致。
func (s *Sender) SendEmailCaptcha(captchaType int, email, code string) error {
	switch captchaType {
	case CaptchaTypeRegister:
		return s.SendRegistrationEmail(email, code)
	case CaptchaTypeResetPassword:
		return s.SendResetPasswordEmail(email, code)
	case CaptchaTypeBindEmail:
		return s.SendBindEmail(email, code)
	default:
		return fmt.Errorf("unsupported email captcha type: %d", captchaType)
	}
}

// SendRegistrationEmail 注册.
func (s *Sender) SendRegistrationEmail(to, code string) error {
	em, err := s.conf()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("【%s】账号注册", em.SendNickName)
	textBody := fmt.Sprintf("您的验证码为：%s，10分钟内有效，请妥善保管。", code)
	htmlBody := buildHTMLTemplate(
		em.SendNickName,
		"账号注册", // 简化大标题，因为上面已经有昵称了
		"欢迎来到我们的世界，您的专属通行证是：",
		code,
		captchaNoteText,
		registerBgImageURL,
		logoIconURL,
		registerCharacterImageURL,
	)

	return s.Send(to, subject, textBody, htmlBody)
}

// SendResetPasswordEmail 重置密码.
func (s *Sender) SendResetPasswordEmail(to, code string) error {
	em, err := s.conf()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("【%s】重置密码", em.SendNickName)
	textBody := fmt.Sprintf("你正在进行密码重置，验证码为：%s，十分钟内有效，请妥善保管。", code)
	htmlBody := buildHTMLTemplate(
		em.SendNickName,
		"重置密码",
		"咦？忘记通行证了？没关系，这是新的通行证：",
		code,
		captchaNoteText,
		registerBgImageURL,
		logoIconURL,
		registerCharacterImageURL,
	)

	return s.Send(to, subject, textBody, htmlBody)
}

// SendBindEmail 绑定邮箱.
func (s *Sender) SendBindEmail(to, code string) error {
	em, err := s.conf()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("【%s】绑定邮箱", em.SendNickName)
	textBody := fmt.Sprintf("你正在进行邮箱绑定，验证码为：%s，十分钟内有效，请妥善保管。", code)
	htmlBody := buildHTMLTemplate(
		em.SendNickName,
		"绑定邮箱",
		"绑定邮箱，让你的账户更安全！这是您的绑定验证码：",
		code,
		captchaNoteText,
		bindBgImageURL,
		logoIconURL,
		bindCharacterImageURL,
	)

	return s.Send(to, subject, textBody, htmlBody)
}
