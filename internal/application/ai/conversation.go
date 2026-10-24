package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
)

const (
	defaultHistoryMaxTurns = 20
	defaultHistoryMaxChars = 24000
	maxInlineImageURLChars = 512
	maxQueryImages         = 3
	imagePlaceholderText   = "[用户上传的图片]"
)

// conversation 是一次请求里已归一化的会话。
type conversation struct {
	Messages         []*schema.Message
	LatestUserText   string
	LatestUserImages []string
}

func (c conversation) HasMessages() bool {
	for _, message := range c.Messages {
		if message != nil && message.Role == schema.User {
			return true
		}
	}

	return false
}

func parseConversation(raw json.RawMessage) (conversation, error) {
	var result conversation

	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		text := strings.TrimSpace(plain)
		if text == "" {
			return result, nil
		}

		result.Messages = []*schema.Message{schema.UserMessage(text)}
		result.LatestUserText = text

		return result, nil
	}

	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return result, fmt.Errorf("解析 input 消息失败: %w", err)
	}

	for _, item := range items {
		role := strings.ToLower(strings.TrimSpace(conversationString(item["role"])))
		text, images := splitContentParts(item["content"])

		messageRole := schema.User
		if role == string(schema.Assistant) {
			messageRole = schema.Assistant
		}

		body := composeMessageText(text, images)
		if body == "" {
			continue
		}

		result.Messages = append(result.Messages, &schema.Message{Role: messageRole, Content: body})
		if messageRole == schema.User {
			result.LatestUserText = text
			result.LatestUserImages = images
		}
	}

	return result, nil
}

func splitContentParts(content any) (string, []string) {
	var texts, images []string

	var walk func(any)
	walk = func(node any) {
		switch value := node.(type) {
		case string:
			if text := strings.TrimSpace(value); text != "" {
				texts = append(texts, text)
			}
		case []any:
			for _, item := range value {
				walk(item)
			}
		case map[string]any:
			switch strings.TrimSpace(conversationString(value["type"])) {
			case "", domain.ContentTypeText, "input_text", "output_text":
				if text := strings.TrimSpace(conversationString(value["text"])); text != "" {
					texts = append(texts, text)
				}
			case domain.ContentTypeImageURL, "input_image", "image":
				if imageURL := conversationImageURL(value); imageURL != "" {
					images = append(images, imageURL)
				}
			}
		}
	}

	walk(content)

	return strings.Join(texts, "\n"), images
}

func conversationImageURL(part map[string]any) string {
	if imageURL := strings.TrimSpace(conversationString(part[domain.ContentTypeImageURL])); imageURL != "" {
		return imageURL
	}
	if nested, ok := part[domain.ContentTypeImageURL].(map[string]any); ok {
		if imageURL := strings.TrimSpace(conversationString(nested["url"])); imageURL != "" {
			return imageURL
		}
	}

	return strings.TrimSpace(conversationString(part["url"]))
}

func composeMessageText(text string, images []string) string {
	if len(images) == 0 {
		return text
	}

	markers := make([]string, 0, len(images))
	for _, imageURL := range images {
		if len(imageURL) <= maxInlineImageURLChars && !strings.HasPrefix(strings.ToLower(imageURL), "data:") {
			markers = append(markers, "[用户上传的图片: "+imageURL+"]")

			continue
		}
		markers = append(markers, imagePlaceholderText)
	}

	marker := strings.Join(markers, " ")
	if text == "" {
		return marker
	}

	return text + "\n" + marker
}

// trimHistory 保留系统消息，并从最新非系统消息向前按消息条数和 rune 文本预算取历史。
// 最新一条即使超出文本预算也会保留；随后去掉开头非 user 消息，因此预算并非严格的总 token 上限。
func (c conversation) trimHistory(cfg *config.AI) conversation {
	maxTurns, maxChars := historyLimits(cfg)

	systemMessages := make([]*schema.Message, 0, len(c.Messages))
	messages := make([]*schema.Message, 0, len(c.Messages))
	for _, message := range c.Messages {
		if message == nil {
			continue
		}
		if message.Role == schema.System {
			systemMessages = append(systemMessages, message)

			continue
		}
		messages = append(messages, message)
	}

	if len(messages) > maxTurns {
		messages = messages[len(messages)-maxTurns:]
	}

	budget := maxChars
	start := len(messages)
	for index := len(messages) - 1; index >= 0; index-- {
		cost := len([]rune(messageText(messages[index])))
		if budget-cost < 0 && index != len(messages)-1 {
			break
		}
		budget -= cost
		start = index
	}
	messages = messages[start:]

	for len(messages) > 0 && messages[0].Role != schema.User {
		messages = messages[1:]
	}

	c.Messages = append(systemMessages, messages...)

	return c
}

func historyLimits(cfg *config.AI) (int, int) {
	maxTurns, maxChars := defaultHistoryMaxTurns, defaultHistoryMaxChars
	if cfg != nil && cfg.HistoryMaxTurns > 0 {
		maxTurns = cfg.HistoryMaxTurns
	}
	if cfg != nil && cfg.HistoryMaxChars > 0 {
		maxChars = cfg.HistoryMaxChars
	}

	return maxTurns, maxChars
}

func messageText(message *schema.Message) string {
	if message == nil {
		return ""
	}
	if message.Content != "" {
		return message.Content
	}

	texts := make([]string, 0, len(message.UserInputMultiContent)+len(message.AssistantGenMultiContent))
	for _, part := range message.UserInputMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	for _, part := range message.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
			texts = append(texts, part.Text)
		}
	}

	return strings.Join(texts, "\n")
}

func (c conversation) prependSystem(text string) conversation {
	text = strings.TrimSpace(text)
	if text == "" {
		return c
	}

	messages := make([]*schema.Message, 0, len(c.Messages)+1)
	messages = append(messages, schema.SystemMessage(text))
	messages = append(messages, c.Messages...)
	c.Messages = messages

	return c
}

// modelMessages 汇合 system 文本并合并相邻同角色消息，为模型适配器构造规范会话。
// 它处理消息形状，不验证模型输出是否可信，也不把文本字符预算换算为真实 tokenizer 计数。
func (c conversation) modelMessages() []*schema.Message {
	var systemTexts []string
	messages := make([]*schema.Message, 0, len(c.Messages))
	for _, message := range c.Messages {
		if message == nil {
			continue
		}
		if message.Role == schema.System {
			if text := strings.TrimSpace(messageText(message)); text != "" {
				systemTexts = append(systemTexts, text)
			}

			continue
		}

		text := strings.TrimSpace(messageText(message))
		if text == "" {
			continue
		}
		if len(messages) > 0 && messages[len(messages)-1].Role == message.Role {
			messages[len(messages)-1].Content += "\n\n" + text

			continue
		}
		messages = append(messages, &schema.Message{Role: message.Role, Content: text})
	}

	if len(systemTexts) == 0 {
		return messages
	}

	result := make([]*schema.Message, 0, len(messages)+1)
	result = append(result, schema.SystemMessage(strings.Join(systemTexts, "\n\n")))

	return append(result, messages...)
}

func (c conversation) queryImages() []string {
	if len(c.LatestUserImages) <= maxQueryImages {
		return c.LatestUserImages
	}

	return c.LatestUserImages[:maxQueryImages]
}

func (c conversation) transcript(maxMessages, maxCharsPerMessage int) string {
	messages := c.Messages
	if maxMessages > 0 && len(messages) > maxMessages {
		messages = messages[len(messages)-maxMessages:]
	}

	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Role == schema.System {
			continue
		}
		speaker := "用户"
		if message.Role == schema.Assistant {
			speaker = "助手"
		}
		lines = append(lines, speaker+": "+domain.TrimPromptText(messageText(message), maxCharsPerMessage))
	}

	return strings.Join(lines, "\n")
}

func conversationString(value any) string {
	text, _ := value.(string)

	return text
}
