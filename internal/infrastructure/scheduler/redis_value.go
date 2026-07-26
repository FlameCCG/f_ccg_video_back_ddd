package scheduler

import "strconv"

// bytesFromRedisValue 把 Lua 脚本返回的单个值归一为字节序列；非字符串类型返回 nil。
func bytesFromRedisValue(raw any) []byte {
	switch v := raw.(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return nil
	}
}

// int64FromRedisValue 把 Lua 脚本返回的单个值归一为 int64；无法解析时返回 0。
func int64FromRedisValue(raw any) int64 {
	switch v := raw.(type) {
	case string:
		val, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return val
		}
	case []byte:
		val, err := strconv.ParseInt(string(v), 10, 64)
		if err == nil {
			return val
		}
	case int64:
		return v
	}

	return 0
}

// toStringSlice 把 Lua 脚本返回的数组归一为字符串切片；非字符串元素被丢弃。
func toStringSlice(raw any) []string {
	switch arr := raw.(type) {
	case []any:
		out := make([]string, 0, len(arr))

		for _, item := range arr {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}

		return out
	case []string:
		return arr
	default:
		return nil
	}
}
