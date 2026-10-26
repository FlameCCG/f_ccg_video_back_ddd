package video

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	app "f_ccg_video_back_ddd/internal/application/video"

	"github.com/redis/go-redis/v9"
)

// Redis TIME 决定北京时间自然日，避免应用节点时钟漂移；同哈希查询先于上限判断。
// 每日集合在该日北京时间次日 00:00 精确过期。
var claimDailyUploadScript = redis.NewScript(`
local now = redis.call('TIME')
local day = math.floor((tonumber(now[1]) + 28800) / 86400)
local key = KEYS[1] .. ':' .. tostring(day)
if redis.call('SISMEMBER', key, ARGV[1]) == 1 then
  return 1
end
if redis.call('SCARD', key) >= tonumber(ARGV[2]) then
  return 0
end
redis.call('SADD', key, ARGV[1])
redis.call('PEXPIREAT', key, ((day + 1) * 86400 - 28800) * 1000)
return 1
`)

// UploadQuotaStore 以 Redis 单脚本原子完成日窗口、哈希去重和名额判定。
// 不同实例共享 Redis TIME；管理员豁免在应用层处理，这里只实现额度占用协议。
type UploadQuotaStore struct {
	redis *redis.Client
}

var _ app.UploadQuotaStore = (*UploadQuotaStore)(nil)

// NewUploadQuotaStore constructs the atomic Redis upload quota store.
func NewUploadQuotaStore(client *redis.Client) *UploadQuotaStore {
	return &UploadQuotaStore{redis: client}
}

// Claim 返回 true 表示该哈希已经或刚刚占有名额，false 表示新哈希被日上限拒绝。
// 已有哈希必须先于上限检查，否则用户耗尽额度后连上传重试也会被错误阻断。
func (s *UploadQuotaStore) Claim(ctx context.Context, userID uint, fileHash string, limit int) (bool, error) {
	if s == nil || s.redis == nil {
		return false, errors.New("redis unavailable")
	}

	if userID == 0 || fileHash == "" || limit <= 0 {
		return false, errors.New("invalid upload quota request")
	}

	key := fmt.Sprintf("video:upload:daily:%d", userID)

	result, err := claimDailyUploadScript.Run(ctx, s.redis, []string{key}, fileHash, strconv.Itoa(limit)).Int()
	if err != nil {
		return false, err
	}

	return result == 1, nil
}
