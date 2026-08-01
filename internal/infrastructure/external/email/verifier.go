package email

import (
	identity "f_ccg_video_back_ddd/internal/application/identity"

	emailverifier "github.com/AfterShip/email-verifier"
)

// AddressVerifier 使用 AfterShip email-verifier 实现邮箱格式与 MX 校验。
type AddressVerifier struct {
	verifier *emailverifier.Verifier
}

var _ identity.EmailAddressVerifier = (*AddressVerifier)(nil)

// NewAddressVerifier 构造与原验证码接口相同配置的邮箱校验器。
func NewAddressVerifier() *AddressVerifier {
	return &AddressVerifier{
		verifier: emailverifier.NewVerifier().
			DisableSMTPCheck().
			DisableCatchAllCheck().
			DisableDomainSuggest().
			DisableGravatarCheck(),
	}
}

// Verify 校验邮箱地址并把第三方结果映射为 identity 上下文的稳定端口模型。
func (v *AddressVerifier) Verify(address string) (*identity.EmailVerifyResult, error) {
	result, err := v.verifier.Verify(address)
	if err != nil {
		return nil, err
	}

	return mapVerifyResult(result), nil
}

func mapVerifyResult(result *emailverifier.Result) *identity.EmailVerifyResult {
	if result == nil {
		return &identity.EmailVerifyResult{}
	}

	mapped := &identity.EmailVerifyResult{
		SyntaxValid:  result.Syntax.Valid,
		HasMXRecords: result.HasMxRecords,
	}
	if result.SMTP != nil {
		mapped.HasSMTPResult = true
		mapped.SMTPHostExists = result.SMTP.HostExists
	}

	return mapped
}
