// Package search 编排关键词搜索、联想、热词、提及和向量索引。
//
// search_service.go 编排搜索与 SQL 回退，vector_service.go/vector_rebuild_service.go 负责向量读写和重建。
// 关键词索引主要由 River 从 binlog 派生，向量索引由应用侧维护；两条更新链路不能视为同一事务。
// 查询 DTO 与写侧实体分开，搜索结果的可见性和回退边界应结合对应端口实现阅读。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package search

import (
	"time"

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

// SearchOptions 搜索参数.
type SearchOptions struct {
	Keyword    string // 搜索关键词
	Page       int    // 页码，从 1 开始
	PageSize   int    // 每页大小
	VideoSort  int    // 视频排序字段：0相关性 1播放量 2弹幕数 3时长
	VideoOrder int    // 视频排序方向：0降序 1升序
	UserSort   int    // 用户排序字段：0相关性 1粉丝数 2等级 3注册时间
	UserOrder  int    // 用户排序方向：0降序 1升序
}

// SearchHighlight 保存字段高亮片段.
type SearchHighlight map[string][]string

// SearchVideoHit 表示综合/视频搜索中的视频命中项.
type SearchVideoHit struct {
	ID             uint            `json:"id"`
	Title          string          `json:"title"`
	Cover          string          `json:"cover"`
	AuthorUsername string          `json:"authorUsername"`
	CreatedAt      time.Time       `json:"createdAt"`
	Highlight      SearchHighlight `json:"highlight,omitempty"`
	Views          int64           `json:"views"`
	DanmuCount     int64           `json:"danmuCount"`
	Duration       int             `json:"duration"`
	Tags           []string        `json:"tags"`
}

// SearchUserHit 表示用户搜索命中项.
type SearchUserHit struct {
	ID            uint            `json:"id"`
	Username      string          `json:"username"`
	Avatar        string          `json:"avatar"`
	Highlight     SearchHighlight `json:"highlight,omitempty"`
	Level         int             `json:"level"`
	FollowerCount int64           `json:"followerCount"`
}

// SearchTagHit 表示标签搜索命中项.
type SearchTagHit struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	Highlight SearchHighlight `json:"highlight,omitempty"`
}

// SuggestionItem 表示搜索联想候选项.
type SuggestionItem struct {
	Value     string `json:"value"`
	Highlight string `json:"highlight"`
	DocType   string `json:"docType"`
}

// SearchResult 表示综合搜索结果.
type SearchResult struct {
	Total      int64            `json:"total"`
	VideoTotal int64            `json:"videoTotal"`
	UserTotal  int64            `json:"userTotal"`
	TagTotal   int64            `json:"tagTotal"`
	Videos     []SearchVideoHit `json:"videos"`
	Users      []SearchUserHit  `json:"users"`
	Tags       []SearchTagHit   `json:"tags"`
}

// VideoSearchResult 表示视频维度搜索结果.
type VideoSearchResult struct {
	VideoTotal int64            `json:"videoTotal"`
	UserTotal  int64            `json:"userTotal"`
	Videos     []SearchVideoHit `json:"videos"`
}

// UserSearchResult 表示用户维度搜索结果.
type UserSearchResult struct {
	UserTotal int64           `json:"userTotal"`
	Users     []SearchUserHit `json:"users"`
}

// HotKeywordItem 首页热搜关键词项.
type HotKeywordItem struct {
	Keyword string `json:"keyword"`
	Score   int64  `json:"score"`
}

// MentionUser 用于评论 @ 联想.
type MentionUser struct {
	ID            uint   `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar"`
	FollowerCount int64  `json:"followerCount"`
	IsFriend      bool   `json:"isFriend"`
}
