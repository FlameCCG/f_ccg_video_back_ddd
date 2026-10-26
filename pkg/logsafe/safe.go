// Package logsafe sanitizes text at the output boundary, including error strings.
package logsafe

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log text bounds count UTF8 bytes; stack text is stored separately.
const (
	// TextLimit bounds ordinary log text to 2 KiB.
	TextLimit  = 2 * 1024
	ErrorLimit = 4 * 1024
)

var (
	email        = regexp.MustCompile(`(?i)[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	urlQuery     = regexp.MustCompile(`(?i)(https?://[^\s?"'<>]+)\?[^\s"'<>]*`)
	urlUser      = regexp.MustCompile(`(?i)(https?://)[^/\s@]+@`)
	sqlText      = regexp.MustCompile(`(?im)\b(?:select\s+[^\r\n]*?\s+from|insert\s+into|update\s+\S+\s+set|delete\s+from)\b[^\r\n]*`)
	cookieHeader = regexp.MustCompile(`(?im)((?:set-cookie|cookie|authorization):)[^\r\n]+`)
	bearer       = regexp.MustCompile(`(?i)\bBearer\s+[a-z0-9._~+/=-]+`)
	assignment   = regexp.MustCompile(`(?i)((?:password|passwd|pwd|token|access_token|refresh_token|api[_-]?key|secret|cookie|authorization|signature|captcha|authcode|验证码)["']?\s*[=:]\s*)(?:"[^"]*"|'[^']*'|[^\s,;&}\]]+)`)
)

// Redactor atomically publishes credential patterns retained across configuration changes.
type Redactor struct {
	secrets atomic.Pointer[[]string]
	mu      sync.Mutex
}

// New creates a sanitizer for configured credentials and known sensitive text forms.
func New(secrets ...string) *Redactor { r := new(Redactor); r.Update(secrets...); return r }

// Update adds credentials without exposing previous credentials from still-running clients.
func (r *Redactor) Update(secrets ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	values := make([]string, 0, len(secrets))
	seen := map[string]bool{}

	if previous := r.secrets.Load(); previous != nil {
		values = append(values, (*previous)...)
		for _, secret := range values {
			seen[secret] = true
		}
	}

	for _, secret := range secrets {
		if secret != "" && !seen[secret] {
			seen[secret] = true
			values = append(values, secret)
		}
	}

	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })

	r.secrets.Store(&values)
}

// Text sanitizes and then truncates without splitting UTF-8 characters.
func (r *Redactor) Text(text string, limit int) string {
	text = urlUser.ReplaceAllString(text, "${1}[REDACTED]@")
	text = urlQuery.ReplaceAllString(text, "${1}?[REDACTED]")
	text = bearer.ReplaceAllString(text, "Bearer [REDACTED]")
	text = cookieHeader.ReplaceAllString(text, "${1}[REDACTED]")
	text = sqlText.ReplaceAllString(text, "[REDACTED_SQL]")
	text = assignment.ReplaceAllString(text, "${1}[REDACTED]")
	text = email.ReplaceAllString(text, "[REDACTED_EMAIL]")

	if r != nil {
		if values := r.secrets.Load(); values != nil {
			for _, secret := range *values {
				if len(secret) < 4 {
					text = regexp.MustCompile(`\b`+regexp.QuoteMeta(secret)+`\b`).ReplaceAllString(text, "[REDACTED]")
				} else {
					text = strings.ReplaceAll(text, secret, "[REDACTED]")
				}
			}
		}
	}

	if limit > 0 && len(text) > limit {
		const suffix = "...[truncated]"

		n := max(limit-len(suffix), 0)

		for n > 0 && !utf8.ValidString(text[:n]) {
			n--
		}

		text = text[:n] + suffix
	}

	return text
}

// Text applies the default sensitive-text rules without process-specific credentials.
func Text(text string, limit int) string { return (*Redactor)(nil).Text(text, limit) }

func sensitive(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	for _, part := range []string{"password", "passwd", "token", "cookie", "authorization", "secret", "apikey", "captcha", "signature"} {
		if strings.Contains(key, part) {
			return true
		}
	}

	switch key {
	case "body", "payload", "prompt", "conversation", "sql", "query", "headers":
		return true
	}

	return false
}

// Core sanitizes both With fields and per-entry fields when they are actually written.
func Core(core zapcore.Core, r *Redactor) zapcore.Core { return &safeCore{Core: core, redactor: r} }

type safeCore struct {
	zapcore.Core
	redactor *Redactor
	fields   []zapcore.Field
}

func (c *safeCore) With(fields []zapcore.Field) zapcore.Core {
	copyCore := *c
	copyCore.fields = append(append([]zapcore.Field{}, c.fields...), fields...)

	return &copyCore
}

func (c *safeCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}

	return checked
}

func (c *safeCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = c.redactor.Text(entry.Message, TextLimit)
	entry.Stack = c.redactor.Text(entry.Stack, 0)
	all := append(append([]zapcore.Field{}, c.fields...), fields...)

	safe := make([]zapcore.Field, 0, len(all))
	for _, field := range all {
		if sensitive(field.Key) {
			safe = append(safe, zap.String(field.Key, "[REDACTED]"))
			continue
		}

		enc := zapcore.NewMapObjectEncoder()

		if field.Type == zapcore.ErrorType {
			if err, ok := field.Interface.(error); ok {
				field = zap.String(field.Key, Error(err))
			}
		}

		field.AddTo(enc)

		for key, value := range enc.Fields {
			limit := TextLimit
			if safeCoreWriteCondition(field, key) {
				limit = ErrorLimit
			}

			if key == "stack" || key == "stacktrace" {
				limit = 0
			}

			safe = append(safe, zap.Any(key, c.clean(value, limit)))
		}
	}

	indexes := map[string]int{}

	unique := make([]zapcore.Field, 0, len(safe))
	for _, field := range safe {
		if index, exists := indexes[field.Key]; exists {
			unique[index] = field
		} else {
			indexes[field.Key] = len(unique)
			unique = append(unique, field)
		}
	}

	if checked := c.Core.Check(entry, nil); checked != nil {
		checked.Write(unique...)
	}

	return nil
}

func (c *safeCore) clean(value any, limit int) any {
	switch v := value.(type) {
	case time.Duration:
		return float64(v) / float64(time.Millisecond)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case string:
		return c.redactor.Text(v, limit)
	case []byte:
		return c.redactor.Text(string(v), limit)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			if sensitive(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = c.clean(item, limit)
			}
		}

		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = c.clean(item, limit)
		}

		return out
	case nil, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	default:
		// Reflect fields and typed collections must be sanitized recursively as well.
		raw, err := json.Marshal(v)
		if err != nil {
			return "[unavailable]"
		}

		var decoded any
		if err = json.Unmarshal(raw, &decoded); err != nil {
			return "[unavailable]"
		}

		return c.clean(decoded, limit)
	}
}

func safeCoreWriteCondition(field zapcore.Field, key string) bool {
	return field.Type == zapcore.ErrorType || strings.Contains(strings.ToLower(key), "error") || strings.Contains(strings.ToLower(key), "cause") || strings.HasSuffix(strings.ToLower(key), "err")
}
