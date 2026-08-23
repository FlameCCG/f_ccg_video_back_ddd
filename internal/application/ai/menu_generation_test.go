package ai

import (
	"context"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type fakeMenuTextGenerator struct {
	invocation TextInvocation
	text       string
}

func (g *fakeMenuTextGenerator) InvokeText(_ context.Context, invocation TextInvocation) (string, error) {
	g.invocation = invocation

	return g.text, nil
}

func (*fakeMenuTextGenerator) OpenStream(
	context.Context,
	TextInvocation,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func TestParseMenuSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name: "valid json in code fence",
			text: "```json\n" +
				`{"titleEn":"Users","titleJa":"ユーザー","icon":"<svg viewBox=\"0 0 24 24\"></svg>",` +
				`"path":"users","name":"Users","component":"views/users/index","keepAlive":true,"sortOrder":2}` +
				"\n```",
		},
		{
			name: "reject event handler in svg",
			text: `{"titleEn":"Users","titleJa":"ユーザー","icon":"<svg onload=\"alert(1)\"></svg>",` +
				`"path":"users","name":"Users","component":"views/users/index","keepAlive":false,"sortOrder":1}`,
			wantErr: true,
		},
		{
			name:    "reject incomplete response",
			text:    `{"titleEn":"Users","sortOrder":1}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			suggestion, err := parseMenuSuggestion(tt.text)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseMenuSuggestion() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("parseMenuSuggestion() error = %v", err)
			}
			if suggestion.SortOrder != 2 || suggestion.Name != "Users" {
				t.Fatalf("unexpected suggestion: %#v", suggestion)
			}
		})
	}
}

func TestServiceGenerateMenuSuggestionBuildsDedicatedConversation(t *testing.T) {
	t.Parallel()

	text := &fakeMenuTextGenerator{text: `{
		"titleEn":"Users","titleJa":"ユーザー","icon":"<svg></svg>",
		"path":"users","name":"Users","component":"views/users/index",
		"keepAlive":false,"sortOrder":2
	}`}
	service := NewService(
		infraconfig.NewProvider(&config.Config{AI: &config.AI{
			ChatModelAPIKey: "test-key",
			ChatModel:       "test-model",
		}}),
		text,
		nil,
		nil,
		nil,
		nil,
		nil, zap.NewNop())

	parentID := uint(9)
	suggestion, err := service.GenerateMenuSuggestion(
		context.Background(),
		"用户管理",
		&parentID,
		[]byte(`[{"id":1,"title":"系统管理"}]`),
	)
	if err != nil {
		t.Fatalf("GenerateMenuSuggestion() error = %v", err)
	}
	if suggestion.Name != "Users" || suggestion.SortOrder != 2 {
		t.Fatalf("unexpected suggestion: %+v", suggestion)
	}
	if text.invocation.Model != "test-model" || text.invocation.MaxTokens != defaultMaxOutputTokens {
		t.Fatalf("unexpected invocation options: %+v", text.invocation)
	}
	if text.invocation.Runtime.ChatModelAPIKey != "test-key" {
		t.Fatalf("runtime config = %+v, want chat key from the captured snapshot", text.invocation.Runtime)
	}
	if len(text.invocation.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(text.invocation.Messages))
	}
	if !strings.Contains(text.invocation.Messages[0].Content, "RBAC 菜单配置生成器") {
		t.Fatalf("unexpected system prompt: %q", text.invocation.Messages[0].Content)
	}
	userPrompt := text.invocation.Messages[1].Content
	for _, want := range []string{"中文标题：用户管理", "父菜单 ID：9", `"title":"系统管理"`} {
		if !strings.Contains(userPrompt, want) {
			t.Fatalf("user prompt %q does not contain %q", userPrompt, want)
		}
	}
}
