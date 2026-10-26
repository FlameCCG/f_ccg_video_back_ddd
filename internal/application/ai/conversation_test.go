package ai

import (
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

const testDataURI = "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD"

func TestParseConversationKeepsHistoryAndSplitsImages(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`[
		{"role":"system","content":"你是助手"},
		{"role":"user","content":[{"type":"input_text","text":"第一句"}]},
		{"role":"assistant","content":[{"type":"output_text","text":"好的"}]},
		{"role":"user","content":[{"type":"input_text","text":"找找这个"},{"type":"input_image","image_url":"` + testDataURI + `"}]}
	]`)

	conv, err := parseConversation(input)
	if err != nil {
		t.Fatalf("parseConversation() error = %v", err)
	}

	if len(conv.Messages) != 4 || conv.Messages[0].Role != schema.User {
		t.Fatalf("messages = %#v, want client system role downgraded to user", conv.Messages)
	}

	if conv.LatestUserText != "找找这个" || len(conv.LatestUserImages) != 1 || conv.LatestUserImages[0] != testDataURI {
		t.Fatalf("conversation = %#v", conv)
	}
}

func TestParseConversationNeverInlinesBase64(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"这是什么"},{"type":"input_image","image_url":"` + testDataURI + `"}]}]`)

	conv, err := parseConversation(input)
	if err != nil {
		t.Fatalf("parseConversation() error = %v", err)
	}

	body := messageText(conv.Messages[len(conv.Messages)-1])
	if strings.Contains(body, "base64") || !strings.Contains(body, imagePlaceholderText) {
		t.Fatalf("message body = %q", body)
	}
}

func TestParseConversationAcceptsPlainString(t *testing.T) {
	t.Parallel()

	conv, err := parseConversation(json.RawMessage(`"你好"`))
	if err != nil {
		t.Fatalf("parseConversation() error = %v", err)
	}

	if len(conv.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(conv.Messages))
	}

	if conv.Messages[0].Role != schema.User || conv.Messages[0].Content != "你好" {
		t.Fatalf("message = %#v, want a plain user message", conv.Messages[0])
	}

	if conv.LatestUserText != "你好" {
		t.Fatalf("LatestUserText = %q, want %q", conv.LatestUserText, "你好")
	}
}

func TestTrimHistoryAppliesTurnWindow(t *testing.T) {
	t.Parallel()

	conv := conversation{Messages: make([]*schema.Message, 0, 10)}

	for index := range 10 {
		role := schema.User
		if index%2 == 1 {
			role = schema.Assistant
		}

		conv.Messages = append(conv.Messages, &schema.Message{
			Role:    role,
			Content: "m",
		})
	}

	trimmed := conv.trimHistory(&config.AI{HistoryMaxTurns: 4, HistoryMaxChars: 1000})
	if len(trimmed.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(trimmed.Messages))
	}

	if trimmed.Messages[0].Role != schema.User {
		t.Fatalf("first role = %q, want %q", trimmed.Messages[0].Role, schema.User)
	}
}

func TestTrimHistoryKeepsLatestTurnAndStartsWithUser(t *testing.T) {
	t.Parallel()

	conv := conversation{Messages: []*schema.Message{
		{Role: schema.User, Content: strings.Repeat("旧", 500)},
		{Role: schema.Assistant, Content: strings.Repeat("答", 500)},
		{Role: schema.User, Content: strings.Repeat("新", 500)},
	}}

	trimmed := conv.trimHistory(&config.AI{HistoryMaxTurns: 20, HistoryMaxChars: 10})
	if len(trimmed.Messages) != 1 || trimmed.Messages[0].Role != schema.User || !strings.Contains(trimmed.Messages[0].Content, "新") {
		t.Fatalf("trimmed = %#v", trimmed.Messages)
	}
}

func TestConversationTranscriptIsBounded(t *testing.T) {
	t.Parallel()

	conv := conversation{Messages: []*schema.Message{
		{Role: schema.User, Content: strings.Repeat("字", 100)},
	}}

	transcript := conv.transcript(6, 10)
	if len([]rune(transcript)) > 30 {
		t.Fatalf("transcript contains %d runes, want at most 30", len([]rune(transcript)))
	}

	if !strings.HasPrefix(transcript, "用户: ") {
		t.Fatalf("transcript = %q, want user prefix", transcript)
	}
}

func TestQueryImagesRespectsCap(t *testing.T) {
	t.Parallel()

	conv := conversation{LatestUserImages: []string{"a", "b", "c", "d", "e"}}

	images := conv.queryImages()
	if len(images) != maxQueryImages {
		t.Fatalf("queryImages() returned %d images, want %d", len(images), maxQueryImages)
	}

	if strings.Join(images, ",") != "a,b,c" {
		t.Fatalf("queryImages() = %#v, want the first three images", images)
	}
}

func TestModelMessagesMergesSystemAndAdjacentRoles(t *testing.T) {
	t.Parallel()

	conv := conversation{Messages: []*schema.Message{
		schema.SystemMessage("系统一"),
		schema.SystemMessage("系统二"),
		schema.UserMessage("问题一"),
		schema.UserMessage("问题二"),
		schema.AssistantMessage("回答", nil),
	}}

	messages := conv.modelMessages()
	if len(messages) != 3 || messages[0].Content != "系统一\n\n系统二" || messages[1].Content != "问题一\n\n问题二" {
		t.Fatalf("messages = %#v", messages)
	}
}
