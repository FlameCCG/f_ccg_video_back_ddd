package search

// 视频排序枚举.
const (
	// VideoSortRelevance 按 ES _score 相关性排序.
	VideoSortRelevance = iota
	// VideoSortViews 按播放量排序.
	VideoSortViews
	// VideoSortDanmu 按弹幕数排序.
	VideoSortDanmu
	// VideoSortDuration 按时长排序.
	VideoSortDuration
)

// 用户排序枚举.
const (
	// UserSortRelevance 按 ES _score 相关性排序.
	UserSortRelevance = iota
	// UserSortFollowers 按粉丝数排序.
	UserSortFollowers
	// UserSortLevel 按等级排序.
	UserSortLevel
	// UserSortCreatedAt 按注册时间排序.
	UserSortCreatedAt
)

// 排序方向.
const (
	// SortOrderDesc 降序.
	SortOrderDesc = 0
	// SortOrderAsc 升序.
	SortOrderAsc = 1
)

// 联想候选项返回给前端的类型标签.
const (
	// DocTypeVideo 视频标题联想.
	DocTypeVideo = "videos"
	// DocTypeUser 用户名联想.
	DocTypeUser = "users"
	// DocTypeTag 标签联想.
	DocTypeTag = "tags"
)
