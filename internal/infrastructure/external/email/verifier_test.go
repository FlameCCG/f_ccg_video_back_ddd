package email

import (
	"testing"

	emailverifier "github.com/AfterShip/email-verifier"
)

func TestMapVerifyResult(t *testing.T) {
	t.Parallel()

	result := mapVerifyResult(&emailverifier.Result{
		Syntax:       emailverifier.Syntax{Valid: true},
		SMTP:         &emailverifier.SMTP{HostExists: true},
		HasMxRecords: true,
	})

	if !result.SyntaxValid || !result.HasSMTPResult || !result.SMTPHostExists || !result.HasMXRecords {
		t.Fatalf("mapVerifyResult() = %#v", result)
	}
}

func TestMapVerifyResultWithoutSMTP(t *testing.T) {
	t.Parallel()

	result := mapVerifyResult(&emailverifier.Result{})
	if result.HasSMTPResult || result.SMTPHostExists {
		t.Fatalf("mapVerifyResult() = %#v, want no SMTP result", result)
	}
}
