package search

import (
	"strconv"
	"strings"
	"unicode"

	searchdomain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/pkg/constants"
)

// esContract 持有 Elasticsearch 的文档 _id、wildcard 转义，以及联想索引选择。
//
// 前缀格式由 River 同步侧写入，属于外部契约。零值即可使用。
type esContract struct{}

// VideoDocumentID 返回视频在搜索/向量索引中的文档 _id（videos:{id}）.
func (esContract) VideoDocumentID(videoID uint) string {
	return "videos:" + strconv.FormatUint(uint64(videoID), 10)
}

// UserDocumentID 返回用户在搜索索引中的文档 _id（users:{id}）.
func (esContract) UserDocumentID(userID uint) string {
	return "users:" + strconv.FormatUint(uint64(userID), 10)
}

// ParseDocumentID 解析 ES 文档 _id（形如 users:12 或 12）中的业务主键。
func (esContract) ParseDocumentID(docID string) (uint, bool) {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return 0, false
	}

	if parts := strings.SplitN(docID, ":", 2); len(parts) == 2 {
		docID = parts[1]
	}

	id, err := strconv.ParseUint(docID, 10, 64)
	if err != nil {
		return 0, false
	}

	return uint(id), true
}

// EscapeWildcard 转义 wildcard 特殊字符并丢弃控制字符；空串返回 false。
//
// 通配查询只在 keyword 子字段上做，避免为中间片段匹配新增 ngram 字段。
func (esContract) EscapeWildcard(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}

	var b strings.Builder
	b.Grow(len(s) + 4)

	for _, r := range s {
		switch r {
		case '\\', '*', '?', '[', ']', '{', '}', '(', ')', '"', ':', '/', '+', '-', '~', '!', '^':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			if unicode.IsControl(r) {
				continue
			}

			b.WriteRune(r)
		}
	}

	out := b.String()
	if out == "" {
		return "", false
	}

	return out, true
}

// SuggestIndex 返回联想查询命中的索引名。
func (esContract) SuggestIndex(docType string) string {
	if docType == searchdomain.DocTypeUser {
		return constants.UserSearchIndex
	}

	return constants.VideoSearchIndex
}
