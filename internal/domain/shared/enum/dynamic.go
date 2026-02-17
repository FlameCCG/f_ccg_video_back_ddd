package enum

// WorkType 作品类型（用于动态模块的“最新作品/已读”判断）.
type WorkType int8

const (
	// WorkTypeVideo 常量.
	WorkTypeVideo WorkType = iota + 1 // 视频
	// WorkTypeDynamic 常量.
	WorkTypeDynamic // 动态（图文）
)
