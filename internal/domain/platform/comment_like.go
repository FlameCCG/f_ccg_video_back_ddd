package platform

// 评论点赞增量队列中的动作取值（与生产端 comment 上下文的取值逐字一致）。
const (
	// LikeActionLike 点赞。
	LikeActionLike = "like"
	// LikeActionUnlike 取消点赞。
	LikeActionUnlike = "unlike"
)

// CommentLikeEvent 评论点赞增量事件。
//
// json tag 是队列消息的外部契约，与生产端写入 Redis 队列的字段逐字一致；
// 消费侧在本包重新声明，避免调度上下文反向依赖 comment 上下文。
type CommentLikeEvent struct {
	// CommentID 评论 ID。
	CommentID uint `json:"commentId"`
	// UserID 操作用户 ID。
	UserID uint `json:"userId"`
	// Action 动作，取值为 LikeActionLike 或 LikeActionUnlike。
	Action string `json:"action"`
}

// CommentLikeIntentKey 折叠键：同一 (评论, 用户) 组合在批内只保留最后意图。
type CommentLikeIntentKey struct {
	// CommentID 评论 ID。
	CommentID uint
	// UserID 操作用户 ID。
	UserID uint
}

// FoldCommentLikeIntents 按入队顺序折叠一批点赞事件：
// 同一 (评论, 用户) 后写覆盖前写，只保留最后一次动作。
//
// 非法事件（评论或用户 ID 为 0、动作不在枚举内）直接丢弃。
// 第二个返回值报告批内是否存在至少一条可应用的意图；
// 为 false 时调用方应直接确认整批以免坏消息堵死队列。
func FoldCommentLikeIntents(events []CommentLikeEvent) (map[CommentLikeIntentKey]string, bool) {
	intents := make(map[CommentLikeIntentKey]string, len(events))

	for _, evt := range events {
		if evt.CommentID == 0 || evt.UserID == 0 {
			continue
		}

		switch evt.Action {
		case LikeActionLike, LikeActionUnlike:
		default:
			continue
		}

		key := CommentLikeIntentKey{CommentID: evt.CommentID, UserID: evt.UserID}
		intents[key] = evt.Action // 后写覆盖前写 = 最后意图
	}

	return intents, len(intents) > 0
}
