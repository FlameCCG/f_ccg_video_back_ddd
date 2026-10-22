package video

import (
	"context"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/constants"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"github.com/redis/go-redis/v9"
)

// ScheduleStore 定时发布任务的 Redis ZSet 适配器
// （原 service/video_service/{scheduler_service.go,update_service.go} 的 ZSet 部分）。
type ScheduleStore struct {
	redis *redis.Client
}

var _ app.LegacyScheduleStore = (*ScheduleStore)(nil)

// NewScheduleStore 构造定时发布任务存储；redisClient 允许为 nil。
func NewScheduleStore(redisClient *redis.Client) *ScheduleStore {
	return &ScheduleStore{redis: redisClient}
}

// Available 报告 Redis 是否可用。
func (s *ScheduleStore) Available() bool {
	return s != nil && s.redis != nil
}

// Add 写入定时发布任务。
//
// 与原 scheduleAt 一致：脱离请求取消（可能在异步 detached 任务里执行），
// 但保留独立的短超时。
func (s *ScheduleStore) Add(ctx context.Context, videoID uint, when time.Time) error {
	if !s.Available() {
		return nil
	}

	opCtx, cancel := ctxutil.WithDetachedTimeout(orTODO(ctx), ctxutil.AsyncQuickTimeout)
	defer cancel()

	return s.redis.ZAdd(opCtx, constants.VideoScheduleKey, redis.Z{
		Score:  float64(when.Unix()),
		Member: scheduleMember(videoID),
	}).Err()
}

// Remove 移除定时发布任务。
func (s *ScheduleStore) Remove(ctx context.Context, videoID uint) error {
	if !s.Available() {
		return nil
	}

	opCtx, cancel := ctxutil.WithDetachedTimeout(orTODO(ctx), ctxutil.AsyncQuickTimeout)
	defer cancel()

	return s.redis.ZRem(opCtx, constants.VideoScheduleKey, scheduleMember(videoID)).Err()
}

// Score 读取定时发布时间戳（Unix 秒）；无任务时返回 (0, false, nil)。
func (s *ScheduleStore) Score(ctx context.Context, videoID uint) (float64, bool, error) {
	if !s.Available() {
		return 0, false, nil
	}

	score, err := s.redis.ZScore(orTODO(ctx), constants.VideoScheduleKey, scheduleMember(videoID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}

		return 0, false, err
	}

	return score, true, nil
}

// DueMembers 读取到期任务的原始 member 列表。
func (s *ScheduleStore) DueMembers(ctx context.Context, nowUnix int64) ([]string, error) {
	if !s.Available() {
		return nil, nil
	}

	return s.redis.ZRangeByScore(orTODO(ctx), constants.VideoScheduleKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: strconv.FormatInt(nowUnix, 10),
	}).Result()
}

// RemoveMember 按原始 member 移除任务（用于清理非法 member）。
func (s *ScheduleStore) RemoveMember(ctx context.Context, member string) error {
	if !s.Available() {
		return nil
	}

	return s.redis.ZRem(orTODO(ctx), constants.VideoScheduleKey, member).Err()
}

// scheduleMember ZSet 成员名（视频 ID 的十进制字符串）。
func scheduleMember(videoID uint) string {
	return strconv.FormatUint(uint64(videoID), 10)
}
