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

// Options 搜索参数.
type Options struct {
	Keyword    string // 搜索关键词
	Page       int    // 页码，从 1 开始
	PageSize   int    // 每页大小
	VideoSort  int    // 视频排序字段：0相关性 1播放量 2弹幕数 3时长
	VideoOrder int    // 视频排序方向：0降序 1升序
	UserSort   int    // 用户排序字段：0相关性 1粉丝数 2等级 3注册时间
	UserOrder  int    // 用户排序方向：0降序 1升序
}

// Highlight 保存字段高亮片段.
type Highlight map[string][]string

// VideoHit 表示综合/视频搜索中的视频命中项.
type VideoHit struct {
	ID             uint      `json:"id"`
	Title          string    `json:"title"`
	Cover          string    `json:"cover"`
	AuthorUsername string    `json:"authorUsername"`
	CreatedAt      time.Time `json:"createdAt"`
	Highlight      Highlight `json:"highlight,omitempty"`
	Views          int64     `json:"views"`
	DanmuCount     int64     `json:"danmuCount"`
	Duration       int       `json:"duration"`
	Tags           []string  `json:"tags"`
}

// UserHit 表示用户搜索命中项.
type UserHit struct {
	ID            uint      `json:"id"`
	Username      string    `json:"username"`
	Avatar        string    `json:"avatar"`
	Highlight     Highlight `json:"highlight,omitempty"`
	Level         int       `json:"level"`
	FollowerCount int64     `json:"followerCount"`
}

// TagHit 表示标签搜索命中项.
type TagHit struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Highlight Highlight `json:"highlight,omitempty"`
}

// SuggestionItem 表示搜索联想候选项.
type SuggestionItem struct {
	Value     string `json:"value"`
	Highlight string `json:"highlight"`
	DocType   string `json:"docType"`
}

// Result 表示综合搜索结果.
type Result struct {
	Total      int64      `json:"total"`
	VideoTotal int64      `json:"videoTotal"`
	UserTotal  int64      `json:"userTotal"`
	TagTotal   int64      `json:"tagTotal"`
	Videos     []VideoHit `json:"videos"`
	Users      []UserHit  `json:"users"`
	Tags       []TagHit   `json:"tags"`
}

// VideoSearchResult 表示视频维度搜索结果.
type VideoSearchResult struct {
	VideoTotal int64      `json:"videoTotal"`
	UserTotal  int64      `json:"userTotal"`
	Videos     []VideoHit `json:"videos"`
}

// UserSearchResult 表示用户维度搜索结果.
type UserSearchResult struct {
	UserTotal int64     `json:"userTotal"`
	Users     []UserHit `json:"users"`
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
