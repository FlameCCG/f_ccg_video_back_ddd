package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeConfiguredModel(t *testing.T) {
	t.Parallel()

	models := []Option{{Label: "V3", Value: "deepseek-v3"}, {Label: "", Value: " "}}

	tests := []struct {
		name      string
		requested string
		want      string
		wantErr   bool
	}{
		{name: "empty falls back", requested: "", want: testDefaultChat},
		{name: "grok falls back", requested: "grok-4-fast", want: testDefaultChat},
		{name: "grok mixed case falls back", requested: "Grok-Beta", want: testDefaultChat},
		{name: "configured default allowed", requested: testDefaultChat, want: testDefaultChat},
		{name: "whitelisted allowed", requested: "deepseek-v3", want: "deepseek-v3"},
		{name: "unknown rejected", requested: "gpt-9", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeConfiguredModel(tt.requested, testDefaultChat, models, "聊天")
			if tt.wantErr {
				require.Error(t, err, "NormalizeConfiguredModel(%q) expected error", tt.requested)

				return
			}

			require.NoError(t, err, "NormalizeConfiguredModel(%q) unexpected error: %v", tt.requested, err)

			require.Equal(t, tt.want, got, "NormalizeConfiguredModel(%q) = %q, want %q", tt.requested, got, tt.want)
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
		{name: "empty uses configured", requested: "", configured: testHigh, want: testHigh},
		{name: "empty uses hard default", requested: "", configured: "", want: testHigh},
		{name: "low maps to high", requested: "low", configured: testHigh, want: testHigh},
		{name: "medium maps to high", requested: "medium", configured: testHigh, want: testHigh},
		{
			name:       "xhigh maps to max when whitelisted",
			requested:  "xhigh",
			configured: testHigh,
			options:    []Option{{Value: "max"}},
			want:       "max",
		},
		{name: "max rejected without whitelist", requested: "max", configured: testHigh, wantErr: true},
		{name: "unknown rejected", requested: "turbo", configured: testHigh, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeConfiguredOption(tt.requested, tt.configured, tt.options, "思考强度", testHigh)
			if tt.wantErr {
				require.Error(t, err, "NormalizeConfiguredOption(%q) expected error", tt.requested)

				return
			}

			require.NoError(t, err, "NormalizeConfiguredOption(%q) unexpected error: %v", tt.requested, err)

			require.Equal(t, tt.want, got, "NormalizeConfiguredOption(%q) = %q, want %q", tt.requested, got, tt.want)
		})
	}
}

func TestCloneOptions(t *testing.T) {
	t.Parallel()

	got := CloneOptions(nil)
	require.NotNil(t, got, "CloneOptions(nil) = %#v, want empty non-nil slice", got)
	require.Empty(t, got, "CloneOptions(nil) = %#v, want empty non-nil slice", got)

	got = CloneOptions([]Option{
		{Label: " 标准 ", Value: " v1 "},
		{Label: "", Value: "v2"},
		{Label: "dropped", Value: "  "},
	})

	want := []Option{{Label: "标准", Value: "v1"}, {Label: "v2", Value: "v2"}}
	require.Len(t, got, len(want), "CloneOptions len = %d, want %d", len(got), len(want))

	for i := range want {
		require.Equal(t, got[i], want[i], "CloneOptions[%d] = %#v, want %#v", i, got[i], want[i])
	}
}

func TestFirstNonEmptyAndURLHelpers(t *testing.T) {
	t.Parallel()

	{
		got := FirstNonEmpty("", "  ", " x ", "y")
		require.Equal(t, "x", got, "FirstNonEmpty = %q, want %q", got, "x")
	}

	{
		got := FirstNonEmpty("", "  ")
		require.Empty(t, got, "FirstNonEmpty with no value = %q, want empty", got)
	}

	{
		got := NormalizeBaseURL(" https://api.example.com/// ")
		require.Equal(t, "https://api.example.com", got, "NormalizeBaseURL = %q", got)
	}

	{
		got := JoinURL(" https://api.example.com/ ", "/v1/messages")
		require.Equal(t, "https://api.example.com/v1/messages", got, "JoinURL = %q", got)
	}
}

func TestAPIErrorMessage(t *testing.T) {
	t.Parallel()

	var nilErr *APIError
	{
		got := nilErr.Error()
		require.Empty(t, got, "nil APIError.Error() = %q, want empty", got)
	}

	withCode := &APIError{StatusCode: 429, Code: "RateLimit", Message: "太快了"}
	{
		got := withCode.Error()
		require.Equal(t, "AI 请求失败[429/RateLimit]: 太快了", got, "APIError.Error() = %q", got)
	}

	withoutCode := &APIError{StatusCode: 500, Message: "内部错误"}
	{
		got := withoutCode.Error()
		require.Equal(t, "AI 请求失败[500]: 内部错误", got, "APIError.Error() = %q", got)
	}
}
