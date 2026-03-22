package search

import (
	searchdomain "f_ccg_video_back_ddd/internal/domain/search"
)

// 联想候选项类型标签（转发领域定义，直接出现在响应 JSON 的 docType 字段）.
const (
	// DocTypeVideo 视频标题联想.
	DocTypeVideo = searchdomain.DocTypeVideo
	// DocTypeUser 用户名联想.
	DocTypeUser = searchdomain.DocTypeUser
	// DocTypeTag 标签联想.
	DocTypeTag = searchdomain.DocTypeTag
)

// SuggestionItem 表示搜索联想候选项.
type SuggestionItem struct {
	Value     string `json:"value"`
	Highlight string `json:"highlight"`
	DocType   string `json:"docType"`
}
