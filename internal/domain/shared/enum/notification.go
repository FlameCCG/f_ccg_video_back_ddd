package enum

// Type 标识通知类别，用于列表分类与未读统计，整数顺序是持久化契约。
type Type int8

const (
	// NotificationApplyType 表示收到回复或作品下评论。
	NotificationApplyType Type = iota + 1 // 回复我的评论的时候或者在我的视频下评论的时候
	// NotificationVideoLikeType 表示视频收到点赞。
	NotificationVideoLikeType Type = iota + 1 // 点赞我的视频的时候
	// NotificationLikeCommentType 表示评论收到点赞。
	NotificationLikeCommentType Type = iota + 1 // 点赞我的评论的时候
	NotificationLikeDynamicType Type = iota + 1 // 点赞我的动态的时候
	NotificationAtType          Type = iota + 1 // @我的时候
	NotificationSystemType      Type = iota + 1 // 系统通知
)
