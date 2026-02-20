package search

import (
	"strconv"
	"strings"
)

// VideoVectorInput 是构造视频向量文本内容所需的业务字段.
type VideoVectorInput struct {
	Title       string
	Description string
	Partition   string
	Tags        []string
	PartTitles  []string
}

// BuildContent 拼装送入 embedding 的文本内容。
//
// 字段顺序与分隔符属于向量库内容契约：重建索引时必须与检索侧一致，
// 否则同一视频在新旧向量空间的表示会漂移。
func (in VideoVectorInput) BuildContent() string {
	contentParts := []string{
		"标题: " + strings.TrimSpace(in.Title),
		"描述: " + strings.TrimSpace(in.Description),
		"分区: " + strings.TrimSpace(in.Partition),
	}

	if tags := TrimNonEmpty(in.Tags); len(tags) > 0 {
		contentParts = append(contentParts, "标签: "+strings.Join(tags, "、"))
	}

	if parts := TrimNonEmpty(in.PartTitles); len(parts) > 0 {
		contentParts = append(contentParts, "分P: "+strings.Join(parts, "、"))
	}

	return strings.Join(contentParts, "\n")
}

// TrimNonEmpty 去掉首尾空白并丢弃空串；全空时返回 nil.
func TrimNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))

	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// VideoDocumentID 返回视频在搜索/向量索引中的文档 _id（videos:{id}）.
//
// 前缀格式由 River 同步侧写入，属于外部契约。
func VideoDocumentID(videoID uint) string {
	return "videos:" + strconv.FormatUint(uint64(videoID), 10)
}

// UserDocumentID 返回用户在搜索索引中的文档 _id（users:{id}）.
func UserDocumentID(userID uint) string {
	return "users:" + strconv.FormatUint(uint64(userID), 10)
}
