package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// jsonKeyContent / jsonKeyRole 消息对象的字段名（上游协议契约）。
const (
	jsonKeyContent = "content"
	jsonKeyRole    = "role"
)

// HasRawJSON 判断原始 JSON 是否有实际内容（非空、非 null）。
func HasRawJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) > 0 && string(trimmed) != "null"
}

// InjectSystemPrompt 在 input 前置注入系统提示词。
//
// 三种输入形态：
//   - 消息数组：首条已是同样内容的 system 消息则原样返回，否则前插一条 system 消息；
//   - 纯文本：包装为 [system, user] 两条消息；
//   - 其它：原样返回。
func InjectSystemPrompt(input json.RawMessage, prompt string) (json.RawMessage, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || !HasRawJSON(input) {
		return input, nil
	}

	var messages []map[string]any
	if err := json.Unmarshal(input, &messages); err == nil {
		if len(messages) > 0 {
			role, _ := messages[0][jsonKeyRole].(string)

			content, _ := messages[0][jsonKeyContent].(string)
			if role == RoleSystem && strings.TrimSpace(content) == prompt {
				return input, nil
			}
		}

		prefixed := make([]map[string]any, 0, len(messages)+1)
		prefixed = append(prefixed, map[string]any{
			jsonKeyRole:    RoleSystem,
			jsonKeyContent: prompt,
		})
		prefixed = append(prefixed, messages...)

		raw, err := json.Marshal(prefixed)
		if err != nil {
			return nil, fmt.Errorf("注入系统提示词失败: %w", err)
		}

		return raw, nil
	}

	var plain string
	if err := json.Unmarshal(input, &plain); err == nil {
		raw, err := json.Marshal([]map[string]any{
			{
				jsonKeyRole:    RoleSystem,
				jsonKeyContent: prompt,
			},
			{
				jsonKeyRole:    "user",
				jsonKeyContent: plain,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("包装文本输入失败: %w", err)
		}

		return raw, nil
	}

	return input, nil
}
