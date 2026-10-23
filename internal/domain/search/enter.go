// Package search 表达搜索词、分页、热词、提及和向量输入规则。
//
// keyword.go/pagination.go 规范输入边界，vector.go 构造向量化文本和封面取材信息。
// 这些函数不连接 Elasticsearch、Redis 或嵌入模型；实际查询、回退、缓存和重建由应用层编排。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
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
