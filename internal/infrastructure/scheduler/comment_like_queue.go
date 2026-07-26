package scheduler

import (
	"context"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/platform"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
)

// commentLikeQueueBatchSize 单批搬运/重放的消息条数（与原实现的 1000 一致）。
const commentLikeQueueBatchSize = 1000

// claimCommentLikeBatchScript 原子地把源队列头部一批消息搬到 processing 队列并返回。
var claimCommentLikeBatchScript = redis.NewScript(`
local source = KEYS[1]
local processing = KEYS[2]
local count = tonumber(ARGV[1] or "0") or 0
local items = {}

for i = 1, count do
  local item = redis.call("LPOP", source)
  if not item then
    break
  end
  redis.call("RPUSH", processing, item)
  table.insert(items, item)
end

return items
`)

// CommentLikeQueue 评论点赞增量队列的 Redis 实现。
//
// Redis key 与原实现完全一致（源队列 comment:like:queue、
// 中转队列 comment:like:queue:processing）。
type CommentLikeQueue struct {
	redis *redis.Client
}

var _ app.CommentLikeQueue = (*CommentLikeQueue)(nil)

// NewCommentLikeQueue 构造评论点赞队列适配器。
func NewCommentLikeQueue(redisClient *redis.Client) *CommentLikeQueue {
	return &CommentLikeQueue{redis: redisClient}
}

// Available 报告底层 Redis 是否已配置。
func (q *CommentLikeQueue) Available() bool {
	return q.redis != nil
}

// NextBatch 取下一批待处理原始消息。
//
// processing 队列有残留（上轮进程异常退出）时优先原样重放，
// 否则用 Lua 原子地从源队列搬运一批到 processing。
func (q *CommentLikeQueue) NextBatch(ctx context.Context) ([]string, error) {
	pending, err := q.redis.LLen(ctx, constants.CommentLikeProcessingQueueKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	if pending > 0 {
		items, readErr := q.redis.LRange(
			ctx, constants.CommentLikeProcessingQueueKey, 0, commentLikeQueueBatchSize-1,
		).Result()
		if readErr != nil && !errors.Is(readErr, redis.Nil) {
			return nil, readErr
		}

		return items, nil
	}

	res, err := claimCommentLikeBatchScript.Run(ctx, q.redis, []string{
		constants.CommentLikeQueueKey,
		constants.CommentLikeProcessingQueueKey,
	}, commentLikeQueueBatchSize).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	return toStringSlice(res), nil
}

// Ack 确认 processing 队列头部 count 条消息已处理完毕（LTRIM 摘除）。
func (q *CommentLikeQueue) Ack(ctx context.Context, count int) error {
	if count <= 0 {
		return nil
	}

	return q.redis.LTrim(ctx, constants.CommentLikeProcessingQueueKey, int64(count), -1).Err()
}
