package search

import (
	searchvo "f_ccg_video_back_ddd/internal/domain/search/valueobject"
)

// 联想候选项类型标签（转发领域定义，直接出现在响应 JSON 的 docType 字段）.
const (
	// DocTypeVideo 视频标题联想.
	DocTypeVideo = searchvo.DocTypeVideo
	// DocTypeUser 用户名联想.
	DocTypeUser = searchvo.DocTypeUser
	// DocTypeTag 标签联想.
	DocTypeTag = searchvo.DocTypeTag
)

// SuggestionItem 表示搜索联想候选项.
type SuggestionItem struct {
	Value     string `json:"value"`
	Highlight string `json:"highlight"`
	DocType   string `json:"docType"`
}
