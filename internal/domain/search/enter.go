// Package search 是搜索限界上下文的领域层：只承载与框架无关的纯业务规则
// （关键词规范化、分页与排序边界、@ 提及匹配规则、热词权重规则、向量化输入构造规则、
// 封面图取材规则），不 import gin / gorm / redis / elastic / viper / zap。
//
// 原实现散落在 service/search_service 的包级函数与常量中，此处集中为可单测的纯逻辑。
package search

// 视频排序枚举（值与原 service/search_service 完全一致，属于对外契约）.
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

// 用户排序枚举（值与原 service/search_service 完全一致，属于对外契约）.
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

// 联想候选项返回给前端的类型标签（对外契约，逐字保持原值）.
const (
	// DocTypeVideo 视频标题联想.
	DocTypeVideo = "videos"
	// DocTypeUser 用户名联想.
	DocTypeUser = "users"
	// DocTypeTag 标签联想.
	DocTypeTag = "tags"
)
