package identity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/redis/go-redis/v9"
)

// 用户索引按到期时间排序；只保留仍在有效期内的 sid，TTL 不缩短其他设备的会话。
const updateSessionIndex = `
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', ARGV[4])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[2])
local latest = redis.call('ZREVRANGE', KEYS[2], 0, 0, 'WITHSCORES')
redis.call('PEXPIREAT', KEYS[2], latest[2])
`

var createSessionScript = redis.NewScript(`
if tonumber(ARGV[3]) <= tonumber(ARGV[4]) then return 0 end
if redis.call('EXISTS', KEYS[1]) == 1 then return 0 end
redis.call('SET', KEYS[1], ARGV[1])
redis.call('PEXPIREAT', KEYS[1], ARGV[3])
` + updateSessionIndex + `return 1`)

// 已消费摘要保留 30 秒以容忍丢失回复和并发刷新；每枚摘要的宽限期只记录一次。
const refreshRetryGrace = 30 * time.Second

var rotateSessionScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if not current then return 0 end
local clock = redis.call('TIME')
local now = tonumber(clock[1]) * 1000 + math.floor(tonumber(clock[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[3], '-inf', now)
-- go-redis 可能在脚本已提交但未收到回复时重试同一组参数。
if current == ARGV[1] or redis.call('ZSCORE', KEYS[3], ARGV[1]) then return 1 end
if current ~= ARGV[5] and not redis.call('ZSCORE', KEYS[3], ARGV[5]) then
  redis.call('DEL', KEYS[1], KEYS[3])
  redis.call('ZREM', KEYS[2], ARGV[2])
  return -1
end
if tonumber(ARGV[3]) <= tonumber(ARGV[4]) then return 0 end
redis.call('ZADD', KEYS[3], 'NX', now + tonumber(ARGV[6]), current)
redis.call('PEXPIREAT', KEYS[3], ARGV[3])
redis.call('SET', KEYS[1], ARGV[1])
redis.call('PEXPIREAT', KEYS[1], ARGV[3])
` + updateSessionIndex + `return 1`)

// SessionStore 使用独立 sid 保存摘要，不保存刷新令牌明文。
type SessionStore struct{ redis *redis.Client }

var _ app.SessionStore = (*SessionStore)(nil)

func NewSessionStore(client *redis.Client) *SessionStore { return &SessionStore{redis: client} }

func (s *SessionStore) keys(userID uint, sessionID string) (string, string) {
	prefix := "auth:session:{" + strconv.FormatUint(uint64(userID), 10) + "}:"
	return prefix + sessionID, prefix + "index"
}

func (s *SessionStore) Create(ctx context.Context, userID uint, sessionID, refreshHash string, expiresAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, ctxutil.RedisWriteTimeout)
	defer cancel()
	key, index := s.keys(userID, sessionID)
	created, err := createSessionScript.Run(ctx, s.redis, []string{key, index}, refreshHash, sessionID, expiresAt.UnixMilli(), time.Now().UnixMilli()).Int()
	if err != nil {
		return fmt.Errorf("create login session: %w", err)
	}
	if created != 1 {
		return errors.New("login session already exists or has expired")
	}
	return nil
}

func (s *SessionStore) Exists(ctx context.Context, userID uint, sessionID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, ctxutil.RedisReadTimeout)
	defer cancel()
	key, _ := s.keys(userID, sessionID)
	count, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("read login session: %w", err)
	}
	return count == 1, nil
}

func (s *SessionStore) Rotate(ctx context.Context, userID uint, sessionID, oldHash, newHash string, expiresAt time.Time) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, ctxutil.RedisWriteTimeout)
	defer cancel()
	key, index := s.keys(userID, sessionID)
	rotated, err := rotateSessionScript.Run(ctx, s.redis, []string{key, index, key + ":previous"}, newHash, sessionID, expiresAt.UnixMilli(), time.Now().UnixMilli(), oldHash, refreshRetryGrace.Milliseconds()).Int()
	if err != nil {
		return false, fmt.Errorf("rotate login session: %w", err)
	}
	if rotated == -1 {
		return false, app.ErrRefreshTokenReused
	}
	return rotated == 1, nil
}

// SnapshotUser 固定待清理会话，调用方应在新密码提交前读取。
func (s *SessionStore) SnapshotUser(ctx context.Context, userID uint) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, ctxutil.RedisReadTimeout)
	defer cancel()
	_, index := s.keys(userID, "")
	ids, err := s.redis.ZRange(ctx, index, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("snapshot user sessions: %w", err)
	}
	return ids, nil
}

// RevokeSessions 原子删除快照中的会话及索引成员，保留之后新建会话的索引与 TTL。
func (s *SessionStore) RevokeSessions(ctx context.Context, userID uint, sessionIDs []string) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, ctxutil.RedisWriteTimeout)
	defer cancel()
	_, index := s.keys(userID, "")
	_, err := s.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		members := make([]any, 0, len(sessionIDs))
		for _, id := range sessionIDs {
			key, _ := s.keys(userID, id)
			pipe.Del(ctx, key, key+":previous")
			members = append(members, id)
		}
		pipe.ZRem(ctx, index, members...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

// RevokeUser 清理本次观察到的会话，不将并发登录纳入重试清理集合。
func (s *SessionStore) RevokeUser(ctx context.Context, userID uint) error {
	ids, err := s.SnapshotUser(ctx, userID)
	if err != nil {
		return err
	}
	return s.RevokeSessions(ctx, userID, ids)
}
