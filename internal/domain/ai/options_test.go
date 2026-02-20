package ai

import "testing"

func TestNormalizeConfiguredModel(t *testing.T) {
	t.Parallel()

	models := []Option{{Label: "V3", Value: "deepseek-v3"}, {Label: "", Value: " "}}

	tests := []struct {
		name      string
		requested string
		want      string
		wantErr   bool
	}{
		{name: "empty falls back", requested: "", want: "default-chat"},
		{name: "grok falls back", requested: "grok-4-fast", want: "default-chat"},
		{name: "grok mixed case falls back", requested: "Grok-Beta", want: "default-chat"},
		{name: "configured default allowed", requested: "default-chat", want: "default-chat"},
		{name: "whitelisted allowed", requested: "deepseek-v3", want: "deepseek-v3"},
		{name: "unknown rejected", requested: "gpt-9", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeConfiguredModel(tt.requested, "default-chat", models, "聊天")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeConfiguredModel(%q) expected error", tt.requested)
				}

				return
			}

			if err != nil {
				t.Fatalf("NormalizeConfiguredModel(%q) unexpected error: %v", tt.requested, err)
			}

			if got != tt.want {
				t.Fatalf("NormalizeConfiguredModel(%q) = %q, want %q", tt.requested, got, tt.want)
			}
		})
	}
}

func TestNormalizeConfiguredOption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		requested  string
		configured string
		options    []Option
		want       string
		wantErr    bool
	}{
		{name: "empty uses configured", requested: "", configured: "high", want: "high"},
		{name: "empty uses hard default", requested: "", configured: "", want: "high"},
		{name: "low maps to high", requested: "low", configured: "high", want: "high"},
		{name: "medium maps to high", requested: "medium", configured: "high", want: "high"},
		{
			name:       "xhigh maps to max when whitelisted",
			requested:  "xhigh",
			configured: "high",
			options:    []Option{{Value: "max"}},
			want:       "max",
		},
		{name: "max rejected without whitelist", requested: "max", configured: "high", wantErr: true},
		{name: "unknown rejected", requested: "turbo", configured: "high", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeConfiguredOption(tt.requested, tt.configured, tt.options, "思考强度", "high")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeConfiguredOption(%q) expected error", tt.requested)
				}

				return
			}

			if err != nil {
				t.Fatalf("NormalizeConfiguredOption(%q) unexpected error: %v", tt.requested, err)
			}

			if got != tt.want {
				t.Fatalf("NormalizeConfiguredOption(%q) = %q, want %q", tt.requested, got, tt.want)
			}
		})
	}
}

func TestCloneOptions(t *testing.T) {
	t.Parallel()

	got := CloneOptions(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("CloneOptions(nil) = %#v, want empty non-nil slice", got)
	}

	got = CloneOptions([]Option{
		{Label: " 标准 ", Value: " v1 "},
		{Label: "", Value: "v2"},
		{Label: "dropped", Value: "  "},
	})

	want := []Option{{Label: "标准", Value: "v1"}, {Label: "v2", Value: "v2"}}
	if len(got) != len(want) {
		t.Fatalf("CloneOptions len = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CloneOptions[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestFirstNonEmptyAndURLHelpers(t *testing.T) {
	t.Parallel()

	if got := FirstNonEmpty("", "  ", " x ", "y"); got != "x" {
		t.Fatalf("FirstNonEmpty = %q, want %q", got, "x")
	}

	if got := FirstNonEmpty("", "  "); got != "" {
		t.Fatalf("FirstNonEmpty with no value = %q, want empty", got)
	}

	if got := NormalizeBaseURL(" https://api.example.com/// "); got != "https://api.example.com" {
		t.Fatalf("NormalizeBaseURL = %q", got)
	}

	if got := JoinURL(" https://api.example.com/ ", "/v1/messages"); got != "https://api.example.com/v1/messages" {
		t.Fatalf("JoinURL = %q", got)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	t.Parallel()

	var nilErr *APIError
	if got := nilErr.Error(); got != "" {
		t.Fatalf("nil APIError.Error() = %q, want empty", got)
	}

	withCode := &APIError{StatusCode: 429, Code: "RateLimit", Message: "太快了"}
	if got := withCode.Error(); got != "AI 请求失败[429/RateLimit]: 太快了" {
		t.Fatalf("APIError.Error() = %q", got)
	}

	withoutCode := &APIError{StatusCode: 500, Message: "内部错误"}
	if got := withoutCode.Error(); got != "AI 请求失败[500]: 内部错误" {
		t.Fatalf("APIError.Error() = %q", got)
	}
}
