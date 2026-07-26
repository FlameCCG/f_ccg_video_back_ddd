package comment

const (
	// LikeActionLike 点赞动作（写入 Redis 回写队列的 action 值）。
	LikeActionLike = "like"
	// LikeActionUnlike 取消点赞动作（写入 Redis 回写队列的 action 值）。
	LikeActionUnlike = "unlike"
)

// LikeEvent 评论点赞回写事件（原 comment_service.CommentLikeEvent）。
//
// 由 Redis Lua 脚本以 JSON 形式 RPUSH 到 constants.CommentLikeQueueKey，
// 再由 platform 上下文的 cron 回写任务消费落库；
// json tag 即队列消息契约，不可更改。
type LikeEvent struct {
	CommentID uint   `json:"commentId"`
	UserID    uint   `json:"userId"`
	Action    string `json:"action"`
}
