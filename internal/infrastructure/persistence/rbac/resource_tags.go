package rbac

import (
	"encoding/json"
	"reflect"
)

// resourceTags 维护资源表 tags 列的已有 JSON 表示及历史数据兼容。
type resourceTags struct{}

func (resourceTags) encode(tags []string) []byte {
	if len(tags) == 0 {
		return nil
	}
	data, _ := json.Marshal(tags) // []string 始终可编码。
	return data
}

func (resourceTags) decode(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var tags []string
	_ = json.Unmarshal(raw, &tags)
	return tags
}

// equal 保留 JSON 语义比较及非法 JSON 的原始字节比较回退。
func (resourceTags) equal(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var left, right any
	if json.Unmarshal(a, &left) != nil || json.Unmarshal(b, &right) != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(left, right)
}
