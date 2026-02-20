// Package search 是搜索限界上下文的应用层：搜索 / 联想 / 热搜词 / @ 提及 /
// 用户索引同步 / 视频向量索引等用例服务，以及它们依赖的端口声明。
//
// 本包不 import gin / gorm / redis / elastic；ES 与 Redis 的访问全部经 ports.go
// 声明的端口，实现分别落在 internal/infrastructure/{search,cache/search,persistence/search}。
package search

import (
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/search"
)

// 视频排序枚举（转发领域定义，供接口层构造入参时使用；值即对外契约）.
const (
	// VideoSortRelevance 0：按 ES _score 相关性排序.
	VideoSortRelevance = domain.VideoSortRelevance
	// VideoSortViews 1：按播放量排序.
	VideoSortViews = domain.VideoSortViews
	// VideoSortDanmu 2：按弹幕数排序.
	VideoSortDanmu = domain.VideoSortDanmu
	// VideoSortDuration 3：按时长排序.
	VideoSortDuration = domain.VideoSortDuration
)

// 用户排序枚举（转发领域定义）.
const (
	// UserSortRelevance 0：按 ES _score 相关性排序.
	UserSortRelevance = domain.UserSortRelevance
	// UserSortFollowers 1：按粉丝数排序.
	UserSortFollowers = domain.UserSortFollowers
	// UserSortLevel 2：按等级排序.
	UserSortLevel = domain.UserSortLevel
	// UserSortCreatedAt 3：按注册时间排序.
	UserSortCreatedAt = domain.UserSortCreatedAt
)

// 排序方向（转发领域定义）.
const (
	// SortOrderDesc 0：降序.
	SortOrderDesc = domain.SortOrderDesc
	// SortOrderAsc 1：升序.
	SortOrderAsc = domain.SortOrderAsc
)

// 联想候选项类型标签（转发领域定义，直接出现在响应 JSON 的 docType 字段）.
const (
	// DocTypeVideo 视频标题联想.
	DocTypeVideo = domain.DocTypeVideo
	// DocTypeUser 用户名联想.
	DocTypeUser = domain.DocTypeUser
	// DocTypeTag 标签联想.
	DocTypeTag = domain.DocTypeTag
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
