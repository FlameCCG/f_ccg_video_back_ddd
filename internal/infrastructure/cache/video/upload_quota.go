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

type UploadQuotaStore struct {
	redis *redis.Client
}

var _ app.UploadQuotaStore = (*UploadQuotaStore)(nil)

func NewUploadQuotaStore(client *redis.Client) *UploadQuotaStore {
	return &UploadQuotaStore{redis: client}
}

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
