package ai

import "github.com/cloudwego/eino/schema"

const toolEventKey = "ai.tool_event"

// ToolCallEvent 只携带可展示信息；不包含原始工具结果、图片数据或上游错误。
type ToolCallEvent struct {
	Phase         string `json:"-"`
	CallID        string `json:"callId"`
	Name          string `json:"name"`
	Label         string `json:"label"`
	Query         string `json:"query,omitempty"`
	ResultCount   int    `json:"resultCount"`
	RetrievalMode string `json:"retrievalMode,omitempty"`
	Message       string `json:"message,omitempty"`
}

// ToolEventMessage 将应用事件放入本地消息流，不透传给模型。
func ToolEventMessage(event ToolCallEvent) *schema.Message {
	return &schema.Message{Extra: map[string]any{toolEventKey: event}}
}

func MessageToolEvent(message *schema.Message) (ToolCallEvent, bool) {
	if message == nil {
		return ToolCallEvent{}, false
	}
	event, ok := message.Extra[toolEventKey].(ToolCallEvent)
	return event, ok
}
