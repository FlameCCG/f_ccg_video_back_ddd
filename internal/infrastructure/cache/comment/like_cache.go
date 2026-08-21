// Package comment 是评论上下文的缓存适配器：评论点赞集合/计数缓存与原子切换脚本。
//
// Redis key 前缀、TTL、Lua 脚本与回写队列 key 均与原 service/comment_service/like_cache.go
// 完全一致，属于外部契约的一部分。
package comment

import (
	"context"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// likeCacheTTL 点赞 set / count 的缓存有效期。
	likeCacheTTL = 24 * time.Hour
	// likeToggleTimeout 点赞原子切换的 Redis 超时。
	likeToggleTimeout = 1 * time.Second
	// likeReadTimeout 批量读取点赞数/点赞状态的 Redis 超时。
	likeReadTimeout = 1 * time.Second
	// likeDropTimeout 写后失效删除 key 的 Redis 超时。
	likeDropTimeout = 500 * time.Millisecond
	// replyCountKeyPrefix 回复数缓存 key 前缀。
	replyCountKeyPrefix = "comment:reply:count:"
)

// toggleLikeScriptSource 点赞原子切换脚本。
//
// 根据 users set 中是否已有该用户自动切换点赞状态，同步维护 count 与 TTL，
// 并把最终意图（like / unlike）推入回写队列供 cron 落库。脚本内容不可改动。
const toggleLikeScriptSource = `
local userKey = KEYS[1]
local countKey = KEYS[2]
local queueKey = KEYS[3]
local userId = ARGV[1]
local commentId = ARGV[2]
local count = tonumber(redis.call("GET", countKey) or "0")

local hasLiked = redis.call("SISMEMBER", userKey, userId) == 1

if hasLiked then
  local removed = redis.call("SREM", userKey, userId)
  if removed == 1 then
    if count > 0 then count = count - 1 end
    redis.call("SET", countKey, count)
    redis.call("PEXPIRE", countKey, tonumber(ARGV[3]))
    if redis.call("EXISTS", userKey) == 1 then
      redis.call("PEXPIRE", userKey, tonumber(ARGV[3]))
    end
    redis.call("RPUSH", queueKey, cjson.encode({commentId=tonumber(commentId), userId=tonumber(userId), action="unlike"}))
  end
  return {count, 0}
else
  local added = redis.call("SADD", userKey, userId)
  if added == 1 then
    count = count + 1
    redis.call("SET", countKey, count)
    redis.call("PEXPIRE", countKey, tonumber(ARGV[3]))
    redis.call("PEXPIRE", userKey, tonumber(ARGV[3]))
    redis.call("RPUSH", queueKey, cjson.encode({commentId=tonumber(commentId), userId=tonumber(userId), action="like"}))
    return {count, 1}
  end
  return {count, 0}
end
`

// LikeCache 评论点赞缓存的 Redis 实现。
//
// 原实现为一组直读 global.RedisClient 的包级函数 + 包级 Lua 脚本变量；
// 新架构收敛为结构体，Redis 客户端与脚本句柄由构造函数注入/初始化。
type LikeCache struct {
	redis  *redis.Client
	toggle *redis.Script
	logger *zap.Logger
}

var _ app.LikeCache = (*LikeCache)(nil)

// NewLikeCache 构造评论点赞缓存适配器。
func NewLikeCache(redisClient *redis.Client, logger *zap.Logger) *LikeCache {
	return &LikeCache{
		redis:  redisClient,
		toggle: redis.NewScript(toggleLikeScriptSource),
		logger: logger,
	}
}

// Available 报告底层 Redis 是否可用。
func (c *LikeCache) Available() bool {
	return c.redis != nil
}

func likeCountKey(commentID uint) string {
	return constants.CommentLikeCountKeyPrefix + strconv.FormatUint(uint64(commentID), 10)
}

func likeUserSetKey(commentID uint) string {
	return constants.CommentLikeUserSetPrefix + strconv.FormatUint(uint64(commentID), 10)
}

func replyCountKey(commentID uint) string {
	return replyCountKeyPrefix + strconv.FormatUint(uint64(commentID), 10)
}

// Complete 判断 count 与 users set 是否成套。
//
// 判定规则：count 存在，且（count==0 或 users set 存在）。
// 不再使用 seeded 标记；用 Set+Count 成套判断。
func (c *LikeCache) Complete(ctx context.Context, commentID uint) (bool, error) {
	if c.redis == nil || commentID == 0 {
		return false, nil
	}

	countKey := likeCountKey(commentID)
	userKey := likeUserSetKey(commentID)

	pipe := c.redis.Pipeline()
	countCmd := pipe.Get(ctx, countKey)
	setExistsCmd := pipe.Exists(ctx, userKey)
	_, _ = pipe.Exec(ctx) // key 缺失时 Get 会带 redis.Nil，属预期

	raw, err := countCmd.Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}

		return false, err
	}

	count, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return false, err
	}

	if count < 0 {
		count = 0
	}

	// count==0 时允许 users set 不存在（真·无人赞）
	if count == 0 {
		return true, nil
	}

	return setExistsCmd.Val() > 0, nil
}

// Write 以给定点赞用户全量重写 set + count（同一 TTL）。
func (c *LikeCache) Write(ctx context.Context, commentID uint, userIDs []uint) error {
	if c.redis == nil {
		return nil
	}

	userKey := likeUserSetKey(commentID)
	countKey := likeCountKey(commentID)
	count := int64(len(userIDs))

	pipe := c.redis.Pipeline()
	pipe.Del(ctx, userKey)

	if len(userIDs) > 0 {
		members := make([]any, len(userIDs))
		for i, id := range userIDs {
			members[i] = id
		}

		pipe.SAdd(ctx, userKey, members...)
		pipe.Expire(ctx, userKey, likeCacheTTL)
	}

	pipe.Set(ctx, countKey, count, likeCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}

// Clear 删除 count/set 两个 key，不写入 count=0。
func (c *LikeCache) Clear(ctx context.Context, commentID uint) error {
	if c.redis == nil || commentID == 0 {
		return nil
	}

	if err := c.redis.Del(ctx, likeCountKey(commentID), likeUserSetKey(commentID)).Err(); err != nil &&
		!errors.Is(err, redis.Nil) {
		return err
	}

	return nil
}

// Drop 尽力删除 count/set 两个 key，失败仅记录日志（用于写后失效）。
func (c *LikeCache) Drop(ctx context.Context, commentID uint) {
	if c.redis == nil {
		return
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeDropTimeout)
	defer cancel()

	if err := c.redis.Del(cacheCtx,
		likeCountKey(commentID),
		likeUserSetKey(commentID),
	).Err(); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("clear comment like cache failed", zap.Uint("commentId", commentID), zap.Error(err))
	}
}

// Toggle 使用 Redis + Lua 保证原子切换（根据当前集合状态自动切换）。
//
// 调用前必须确保 set/count 成套（见 Complete / Write / Clear）。
func (c *LikeCache) Toggle(ctx context.Context, commentID, userID uint) (int64, bool, error) {
	if c.redis == nil {
		return 0, false, errors.New("redis not configured")
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeToggleTimeout)
	defer cancel()

	ttlMs := likeCacheTTL.Milliseconds()
	if ttlMs <= 0 {
		ttlMs = 1
	}

	res, err := c.toggle.Run(cacheCtx, c.redis, []string{
		likeUserSetKey(commentID),
		likeCountKey(commentID),
		constants.CommentLikeQueueKey,
	}, strconv.FormatUint(uint64(userID), 10),
		strconv.FormatUint(uint64(commentID), 10),
		strconv.FormatInt(ttlMs, 10),
	).Result()
	if err != nil {
		return 0, false, err
	}

	return parseToggleScriptResult(res)
}

// Counts 批量获取点赞数缓存（整体失败时返回 nil 触发回退）。
//
// 仅返回 count key 存在的项；缺失由调用方用 DB 字段兜底，并异步重建。
func (c *LikeCache) Counts(ctx context.Context, commentIDs []uint) (map[uint]int64, []uint) {
	if c.redis == nil || len(commentIDs) == 0 {
		return nil, append([]uint(nil), commentIDs...)
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeReadTimeout)
	defer cancel()

	keys := make([]string, 0, len(commentIDs))
	for _, id := range commentIDs {
		keys = append(keys, likeCountKey(id))
	}

	values, err := c.redis.MGet(cacheCtx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("fetch comment like count cache failed", zap.Error(err))

		return nil, append([]uint(nil), commentIDs...)
	}

	result := make(map[uint]int64, len(commentIDs))
	missIDs := make([]uint, 0)

	for i, raw := range values {
		if raw == nil {
			missIDs = append(missIDs, commentIDs[i])

			continue
		}

		switch v := raw.(type) {
		case string:
			if val, convErr := strconv.ParseInt(v, 10, 64); convErr == nil {
				result[commentIDs[i]] = val
			} else {
				missIDs = append(missIDs, commentIDs[i])
			}
		case []byte:
			if val, convErr := strconv.ParseInt(string(v), 10, 64); convErr == nil {
				result[commentIDs[i]] = val
			} else {
				missIDs = append(missIDs, commentIDs[i])
			}
		default:
			missIDs = append(missIDs, commentIDs[i])
		}
	}

	return result, missIDs
}

// LikedFlags 批量判断点赞状态。
//
// 缓存不完整（count 缺失，或 count>0 但 set 缺失）记为 miss；
// 整体失败返回 nil 触发全量回源。
func (c *LikeCache) LikedFlags(
	ctx context.Context,
	commentIDs []uint,
	userID uint,
) (map[uint]bool, []uint) {
	if c.redis == nil {
		return nil, append([]uint(nil), commentIDs...)
	}

	if userID == 0 || len(commentIDs) == 0 {
		return map[uint]bool{}, nil
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeReadTimeout)
	defer cancel()

	// 批量探测 count / set 是否成套
	type probe struct {
		countGet  *redis.StringCmd
		setExists *redis.IntCmd
	}

	pipe := c.redis.Pipeline()
	probes := make(map[uint]probe, len(commentIDs))

	for _, id := range commentIDs {
		probes[id] = probe{
			countGet:  pipe.Get(cacheCtx, likeCountKey(id)),
			setExists: pipe.Exists(cacheCtx, likeUserSetKey(id)),
		}
	}

	// 允许部分 key 缺失（Get 带 redis.Nil），逐条读取结果时再判定 miss。
	_, _ = pipe.Exec(cacheCtx)

	hitIDs := make([]uint, 0, len(commentIDs))
	missIDs := make([]uint, 0)

	for _, id := range commentIDs {
		p := probes[id]

		raw, err := p.countGet.Result()
		if err != nil {
			missIDs = append(missIDs, id)

			continue
		}

		count, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			missIDs = append(missIDs, id)

			continue
		}

		if count > 0 && p.setExists.Val() == 0 {
			// 半残：有 count 无 set
			missIDs = append(missIDs, id)

			continue
		}

		hitIDs = append(hitIDs, id)
	}

	result := make(map[uint]bool, len(commentIDs))
	if len(hitIDs) == 0 {
		return result, missIDs
	}

	pipe2 := c.redis.Pipeline()
	cmds := make(map[uint]*redis.BoolCmd, len(hitIDs))

	for _, id := range hitIDs {
		// count==0 且无 set：SISMEMBER 对不存在 key 为 false，正确表示未赞
		cmds[id] = pipe2.SIsMember(cacheCtx, likeUserSetKey(id), userID)
	}

	if _, err := pipe2.Exec(cacheCtx); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("fetch comment like flags cache failed", zap.Error(err))

		return nil, append([]uint(nil), commentIDs...)
	}

	for id, cmd := range cmds {
		if cmd.Err() != nil {
			missIDs = append(missIDs, id)

			continue
		}

		if cmd.Val() {
			result[id] = true
		}
	}

	return result, missIDs
}

// ReplyCounts 批量获取回复数缓存；缺失项由调用方用 DB 字段兜底。
func (c *LikeCache) ReplyCounts(ctx context.Context, commentIDs []uint) map[uint]int64 {
	result := make(map[uint]int64)
	if c.redis == nil || len(commentIDs) == 0 {
		return result
	}

	pipe := c.redis.Pipeline()

	cmds := make(map[uint]*redis.StringCmd, len(commentIDs))
	for _, id := range commentIDs {
		cmds[id] = pipe.Get(ctx, replyCountKey(id))
	}

	_, _ = pipe.Exec(ctx)

	for id, cmd := range cmds {
		if cmd.Err() == nil {
			if val, err := cmd.Int64(); err == nil {
				result[id] = val
			}
		}
	}

	return result
}

// parseToggleScriptResult 解析 Lua 返回的 {count, isLiked} 二元组。
func parseToggleScriptResult(res any) (int64, bool, error) {
	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return 0, false, errors.New("invalid lua result")
	}

	count, err := toInt64(arr[0])
	if err != nil {
		return 0, false, err
	}

	isLiked := toBool(arr[1])

	return count, isLiked, nil
}

func toInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int64:
		return val, nil
	case int:
		return int64(val), nil
	case float64:
		return int64(val), nil
	case string:
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, errors.New("unexpected number type")
	}
}

func toBool(v any) bool {
	switch val := v.(type) {
	case int64:
		return val != 0
	case int:
		return val != 0
	case float64:
		return val != 0
	case bool:
		return val
	default:
		return false
	}
}
