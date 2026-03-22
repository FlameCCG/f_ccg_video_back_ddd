package identity

import (
	"testing"
	"time"
)

func TestEmailCaptchaEligibilityPreservesLookupFailureBehavior(t *testing.T) {
	p := EmailCaptchaPolicy{}
	for _, tc := range []struct {
		purpose         int
		found, lookupOK bool
		want            error
	}{
		{EmailCaptchaRegister, true, true, ErrEmailAlreadyRegistered},
		{EmailCaptchaRegister, true, false, nil},
		{EmailCaptchaBinding, true, true, ErrEmailAlreadyBound},
		{EmailCaptchaBinding, true, false, nil},
		{EmailCaptchaPasswordRecovery, false, true, ErrEmailCaptchaEmailNotRegistered},
		{EmailCaptchaPasswordRecovery, true, false, ErrEmailCaptchaEmailNotRegistered},
		{EmailCaptchaPasswordRecovery, true, true, nil},
		{99, false, false, nil},
	} {
		if err := p.EnsureAccount(tc.purpose, tc.found, tc.lookupOK); err != tc.want {
			t.Errorf("%+v err=%v", tc, err)
		}
	}
	f := EmailAddressFacts{HasSMTPResult: true}
	if f.EnsureReachable() != ErrEmailSyntaxInvalid {
		t.Fatal("syntax must take precedence")
	}
	f.SyntaxValid = true
	if f.EnsureReachable() != ErrEmailHostNotExists {
		t.Fatal("SMTP host must take precedence over MX")
	}
	f.HasSMTPResult = false
	if f.EnsureReachable() != ErrEmailNoMXRecords {
		t.Fatal("absent SMTP result must not reject the host")
	}
	f.HasMXRecords = true
	if err := f.EnsureReachable(); err != nil {
		t.Fatal(err)
	}
	s := p.Sending()
	if s.Window != 10*time.Minute || s.PerEmail != 5 || s.PerIP != 20 || s.CodeLength != 5 || s.Charset != "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("send policy changed: %+v", s)
	}
}
