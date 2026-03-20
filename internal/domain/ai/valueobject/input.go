package valueobject

import (
	"bytes"
	"encoding/json"
)

// HasRawJSON 判断原始 JSON 是否有实际内容（非空、非 null）。
func HasRawJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) > 0 && string(trimmed) != "null"
}
