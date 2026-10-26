package logsafe

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestOutputSanitizesErrorsAndNestedValues(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	redactor := New("configured-secret")
	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&output), zap.DebugLevel)
	logger := zap.New(Core(core, redactor)).With(zap.String("email", "person@example.com"))
	err := fmt.Errorf("dependency configured-secret token=tokenvalue: %w", &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'sql-value-must-not-log'"})
	logger.Error("failed", zap.Error(err), zap.String("url", "https://host/path?signature=signed-value"), zap.Any("nested", map[string]string{"accessToken": "nested-secret", "email": "nested@example.com"}), zap.String("Cookie", "session=cookie-value"))

	text := output.String()
	for _, secret := range []string{"configured-secret", "tokenvalue", "sql-value-must-not-log", "signed-value", "nested-secret", "cookie-value", "person@example.com", "nested@example.com"} {
		if strings.Contains(text, secret) {
			t.Fatalf("leaked %q: %s", secret, text)
		}
	}

	if !strings.Contains(text, "mysql error 1062") {
		t.Fatalf("lost non-sensitive diagnostic: %s", text)
	}
}

func TestTextLimitsAndReloadSecretHistory(t *testing.T) {
	t.Parallel()

	r := New("old-secret")
	r.Update("new-secret")

	text := r.Text("old-secret new-secret", TextLimit)
	if strings.Contains(text, "old-secret") || strings.Contains(text, "new-secret") {
		t.Fatal("reload exposed a configured credential")
	}

	for _, limit := range []int{TextLimit, ErrorLimit} {
		text = r.Text(strings.Repeat("长", limit), limit)
		if len(text) > limit || !utf8.ValidString(text) || !strings.HasSuffix(text, "[truncated]") {
			t.Fatalf("invalid truncation length=%d", len(text))
		}
	}

	if got := Text(`{"token":"json-token-secret"}`, TextLimit); strings.Contains(got, "json-token-secret") {
		t.Fatalf("JSON credential leaked: %s", got)
	}

	if got := Text("Bearer bearer-secret", TextLimit); strings.Contains(got, "bearer-secret") {
		t.Fatal("Bearer token leaked")
	}

	if Error(errors.New("safe")) != "safe" {
		t.Fatal("safe error changed")
	}
}

func TestMillisecondsUTCAndInheritedFieldsRemainUnambiguous(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer

	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&output), zap.InfoLevel)
	logger := zap.New(Core(core, New())).With(zap.String("component", "application"))
	logger.With(zap.String("component", "http")).Info("done", zap.Duration("duration_ms", 2500*time.Microsecond), zap.Time("completed_at", time.Date(2026, 10, 24, 12, 0, 0, 0, time.FixedZone("offset", 3600))))

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}

	if record["duration_ms"] != 2.5 || record["completed_at"] != "2026-10-24T11:00:00Z" || record["component"] != "http" || strings.Count(output.String(), `"component"`) != 1 {
		t.Fatalf("ambiguous output fields: %s", output.String())
	}
}

func TestShortCredentialDoesNotBreakURLRedaction(t *testing.T) {
	t.Parallel()

	text := New("p").Text("https://host/path?custom=private-url-value", TextLimit)
	if strings.Contains(text, "private-url-value") {
		t.Fatalf("short credential disabled URL redaction: %s", text)
	}
}

func TestCapturedProcessStderrIsInspectableAndSanitized(t *testing.T) {
	t.Parallel()

	process := &exec.ExitError{ProcessState: &os.ProcessState{}, Stderr: []byte("probe failed token=stderr-private-value")}
	wrapped := fmt.Errorf("ffprobe failed: %w", process)

	var original *exec.ExitError
	if !errors.As(wrapped, &original) || original != process {
		t.Fatal("original process failure lost")
	}

	var output bytes.Buffer

	core := zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&output), zap.ErrorLevel)
	zap.New(Core(core, New())).Error("failed", zap.Error(wrapped))

	if !strings.Contains(output.String(), "probe failed") || strings.Contains(output.String(), "stderr-private-value") {
		t.Fatalf("unsafe or missing process diagnosis: %s", output.String())
	}
}
