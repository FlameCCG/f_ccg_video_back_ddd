package ai

import (
	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"testing"
)

// 验证 FirstNonEmpty 与 URL 辅助函数的全部行为。
func TestFirstNonEmptyAndURLHelpers(t *testing.T) {
	t.Parallel()
	if got := (aidomain.PromptVideoCandidate{}).FirstNonEmpty("", "  ", " x ", "y"); got != "x" {
		t.Fatalf("FirstNonEmpty = %q, want x", got)
	}
	if got := (aidomain.PromptVideoCandidate{}).FirstNonEmpty("", "  "); got != "" {
		t.Fatalf("FirstNonEmpty = %q, want empty", got)
	}
	source := &config.Config{AI: &config.AI{ChatModelBaseURL: " https://api.example.com/// "}}
	svc := &Service{cfgProvider: infraconfig.NewProvider(source)}
	snapshot, err := svc.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.runtime.ChatModelBaseURL != "https://api.example.com" || snapshot.settings.ChatModelBaseURL != "https://api.example.com" {
		t.Fatal(snapshot)
	}
	if source.AI.ChatModelBaseURL != " https://api.example.com/// " {
		t.Fatal("normalization changed shared configuration")
	}
}
