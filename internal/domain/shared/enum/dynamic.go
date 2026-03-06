package enum

// WorkType 作品类型（用于动态模块的“最新作品/已读”判断）.
type WorkType int8

const (
	// WorkTypeVideo 标记活动/已读记录对应视频作品。
	WorkTypeVideo WorkType = iota + 1 // 视频
	// WorkTypeDynamic 标记活动/已读记录对应图文动态。
	WorkTypeDynamic // 动态（图文）
)
