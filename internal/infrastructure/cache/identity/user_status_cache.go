// Package identity 是身份上下文的缓存适配器：用户状态缓存与创作中心指标存储。
package identity

import (
	"context"
	"errors"
	"strconv"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 短 TTL：降低异常窗口；多实例一致依赖 Redis。
// generation CAS 阻止在途旧 Active 在封禁后写回。
const (
	userStatusCacheTTL     = 60 * time.Second
	userStatusGenTTL       = 2 * time.Minute
	userStatusRedisTimeout = 200 * time.Millisecond
)

// storeUserStatusIfGenMatchScript：仅当 generation 仍等于 load 前快照时才回填。
// 封禁/解封 write-through 会 bump gen，使在途旧 Active 回填失败。
var storeUserStatusIfGenMatchScript = redis.NewScript(`
local genKey = KEYS[1]
local statusKey = KEYS[2]
local expected = tonumber(ARGV[1] or "0") or 0
local status = ARGV[2]
local ttl = tonumber(ARGV[3] or "0") or 0

local current = tonumber(redis.call("GET", genKey) or "0") or 0
if current ~= expected then
  return 0
end

if ttl > 0 then
  redis.call("SET", statusKey, status, "EX", ttl)
else
  redis.call("SET", statusKey, status)
end
return 1
`)

// UserStatusCache 用户状态缓存的 Redis 实现（原 service/user_service/status_cache.go）。
//
// Redis key 格式与 TTL 与原实现完全一致（user:status: / user:status:gen:）。
type UserStatusCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var _ domain.UserStatusCache = (*UserStatusCache)(nil)

// NewUserStatusCache 构造用户状态缓存适配器。
func NewUserStatusCache(redisClient *redis.Client, logger *zap.Logger) *UserStatusCache {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &UserStatusCache{redis: redisClient, logger: logger}
}

func userStatusRedisKey(userID uint) string {
	return constants.UserStatusCacheKeyPrefix + strconv.FormatUint(uint64(userID), 10)
}

func userStatusGenRedisKey(userID uint) string {
	return constants.UserStatusGenKeyPrefix + strconv.FormatUint(uint64(userID), 10)
}

// Load 读取缓存中的用户状态；未命中或脏数据返回 (0, false)。
func (c *UserStatusCache) Load(ctx context.Context, userID uint) (enum.UserStatus, bool) {
	if c.redis == nil {
		return 0, false
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cacheCtx, cancel := context.WithTimeout(ctx, userStatusRedisTimeout)
	defer cancel()

	raw, err := c.redis.Get(cacheCtx, userStatusRedisKey(userID)).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get user status cache failed", zap.Uint("userId", userID), zap.Error(err))
		}

		return 0, false
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		// 脏数据直接删除并 miss
		_ = c.redis.Del(cacheCtx, userStatusRedisKey(userID)).Err()

		return 0, false
	}

	return enum.UserStatus(n), true
}

// Generation 读取当前 generation 快照。
func (c *UserStatusCache) Generation(ctx context.Context, userID uint) int64 {
	if c.redis == nil || userID == 0 {
		return 0
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cacheCtx, cancel := context.WithTimeout(ctx, userStatusRedisTimeout)
	defer cancel()

	val, err := c.redis.Get(cacheCtx, userStatusGenRedisKey(userID)).Int64()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger.Warn("get user status generation failed", zap.Uint("userId", userID), zap.Error(err))
		}

		return 0
	}

	return val
}

// StoreIfGenerationMatch 仅在 generation 未变化时回填，防止在途旧 Active 覆盖封禁结果。
func (c *UserStatusCache) StoreIfGenerationMatch(
	ctx context.Context,
	userID uint,
	status enum.UserStatus,
	expectedGen int64,
) {
	if c.redis == nil || userID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cacheCtx, cancel := context.WithTimeout(ctx, userStatusRedisTimeout)
	defer cancel()

	ttlSeconds := int(userStatusCacheTTL / time.Second)

	ok, err := storeUserStatusIfGenMatchScript.Run(cacheCtx, c.redis, []string{
		userStatusGenRedisKey(userID),
		userStatusRedisKey(userID),
	}, strconv.FormatInt(expectedGen, 10), strconv.Itoa(int(status)), strconv.Itoa(ttlSeconds)).Int()
	if err != nil {
		c.logger.Warn("set user status cache failed", zap.Uint("userId", userID), zap.Error(err))

		return
	}

	if ok == 0 {
		c.logger.Debug("skip user status cache write: generation changed",
			zap.Uint("userId", userID), zap.Int64("gen", expectedGen))
	}
}

// Write 权威写穿：直接 SET 最新状态并 bump generation。
func (c *UserStatusCache) Write(userID uint, status enum.UserStatus) {
	if c.redis == nil || userID == 0 {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), userStatusRedisTimeout)
	defer cancel()

	statusKey := userStatusRedisKey(userID)
	genKey := userStatusGenRedisKey(userID)

	pipe := c.redis.Pipeline()
	pipe.Incr(cacheCtx, genKey)
	pipe.Expire(cacheCtx, genKey, userStatusGenTTL)
	pipe.Set(cacheCtx, statusKey, strconv.Itoa(int(status)), userStatusCacheTTL)

	if _, err := pipe.Exec(cacheCtx); err != nil {
		c.logger.Warn("write user status cache failed", zap.Uint("userId", userID), zap.Error(err))
	}
}

// Invalidate 删除 Redis 中的状态缓存并 bump generation。
func (c *UserStatusCache) Invalidate(userID uint) {
	if c.redis == nil || userID == 0 {
		return
	}

	cacheCtx, cancel := context.WithTimeout(context.Background(), userStatusRedisTimeout)
	defer cancel()

	statusKey := userStatusRedisKey(userID)
	genKey := userStatusGenRedisKey(userID)

	pipe := c.redis.Pipeline()
	pipe.Del(cacheCtx, statusKey)
	pipe.Incr(cacheCtx, genKey)
	pipe.Expire(cacheCtx, genKey, userStatusGenTTL)

	if _, err := pipe.Exec(cacheCtx); err != nil {
		c.logger.Warn("invalidate user status cache failed", zap.Uint("userId", userID), zap.Error(err))
	}
}
