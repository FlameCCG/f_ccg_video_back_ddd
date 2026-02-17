package enum

// Type 类型.
type Type int8

// NotificationLikeCommentType 及相关导出常量.
const (
	// NotificationApplyType 常量.
	NotificationApplyType Type = iota + 1 // 回复我的评论的时候或者在我的视频下评论的时候
	// NotificationVideoLikeType 常量.
	NotificationVideoLikeType Type = iota + 1 // 点赞我的视频的时候
	// NotificationLikeCommentType 常量.
	NotificationLikeCommentType Type = iota + 1 // 点赞我的评论的时候
	NotificationLikeDynamicType Type = iota + 1 // 点赞我的动态的时候
	NotificationAtType          Type = iota + 1 // @我的时候
	NotificationSystemType      Type = iota + 1 // 系统通知
)
