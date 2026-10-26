package search

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
)

// 本文件是 ES `_source` 的弱类型解码工具：同一字段在 dump / binlog / 手工写入下
// 可能是 string、json.Number、float64 等不同类型，逐类型兜底转换而不是直接断言。

func toHighlightMap(source map[string][]string) app.Highlight {
	if len(source) == 0 {
		return nil
	}

	out := make(app.Highlight, len(source))
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

func toUint(value any) uint {
	switch v := value.(type) {
	case json.Number:
		n, err := strconv.ParseUint(v.String(), 10, strconv.IntSize)
		if err != nil || n > math.MaxUint {
			return 0
		}

		return uint(n)
	case int64:
		return nonnegativeUint(v)
	case int:
		return nonnegativeUint(int64(v))
	case float64:
		return floatUint(v)
	case float32:
		return floatUint(float64(v))
	case uint64:
		if v > math.MaxUint {
			return 0
		}

		return uint(v)
	case uint:
		return v
	default:
		return 0
	}
}

func toInt64(value any) int64 {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0
		}

		return n
	case float64:
		return floatInt64(v)
	case float32:
		return floatInt64(float64(v))
	case int64:
		return v
	case int:
		return int64(v)
	case uint64:
		if v > math.MaxInt64 {
			return 0
		}

		return int64(v)
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0
		}

		return int64(v)
	default:
		return 0
	}
}

func toInt(value any) int {
	n := toInt64(value)
	if n < math.MinInt || n > math.MaxInt {
		return 0
	}

	return int(n)
}

func floatUint(value float64) uint {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value >= float64(math.MaxUint) {
		return 0
	}

	return uint(value)
}

func floatInt64(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < float64(math.MinInt64) || value >= float64(math.MaxInt64) {
		return 0
	}

	return int64(value)
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

// nonnegativeUint rejects negative and architecture-dependent overflow values.
func nonnegativeUint(value int64) uint {
	if value < 0 || uint64(value) > uint64(math.MaxUint) {
		return 0
	}

	return uint(value)
}
