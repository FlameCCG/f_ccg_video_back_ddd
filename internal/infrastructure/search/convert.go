package search

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
)

// 本文件是 ES `_source` 的弱类型解码工具：同一字段在 dump / binlog / 手工写入下
// 可能是 string、json.Number、float64 等不同类型，逐类型兜底转换而不是直接断言。

func toHighlightMap(source map[string][]string) app.SearchHighlight {
	if len(source) == 0 {
		return nil
	}

	out := make(app.SearchHighlight, len(source))
	maps.Copy(out, source)

	return out
}

func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case float64, float32, int64, int, uint64, uint:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}

func toTime(val any) time.Time {
	switch v := val.(type) {
	case time.Time:
		return v
	case string:
		return parseTimeString(v)
	case []byte:
		return parseTimeString(string(v))
	case json.Number:
		if unix, err := v.Int64(); err == nil {
			return time.Unix(unix, 0)
		}
	case float64:
		return time.Unix(int64(v), 0)
	case int64:
		return time.Unix(v, 0)
	case int:
		return time.Unix(int64(v), 0)
	}

	return time.Time{}
}

func parseTimeString(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}

	return time.Time{}
}

func toUint(val any) uint {
	switch v := val.(type) {
	case json.Number:
		i, _ := v.Int64()

		return uint(i)
	case float64:
		return uint(v)
	case float32:
		return uint(v)
	case int64:
		return uint(v)
	case int:
		return uint(v)
	case uint64:
		return uint(v)
	case uint:
		return v
	default:
		return 0
	}
}

func toInt64(val any) int64 {
	switch v := val.(type) {
	case json.Number:
		i, _ := v.Int64()

		return i
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case uint:
		return int64(v)
	default:
		return 0
	}
}

func toInt(val any) int {
	switch v := val.(type) {
	case json.Number:
		i, _ := v.Int64()

		return int(i)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case int64:
		return int(v)
	case int:
		return v
	case uint64:
		return int(v)
	case uint:
		return int(v)
	default:
		return 0
	}
}

func toStringSlice(val any) []string {
	switch v := val.(type) {
	case []any:
		out := make([]string, 0, len(v))

		for _, item := range v {
			if s := toString(item); s != "" {
				out = append(out, s)
			}
		}

		return out
	case []string:
		return v
	case string:
		if v == "" {
			return []string{}
		}

		return domain.SplitTags(v)
	default:
		return []string{}
	}
}

func toFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()

		return n
	default:
		return 0
	}
}
