package search

import (
	"time"
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
