// Package media 适配转码进度、取消广播与详情缓存失效。
//
// progress_cache.go 保存进度快照并发布变更，cancel_broadcaster.go 发送删除取消信号。
// Pub/Sub 通知是最佳努力信号，快照可补读；消费者还会核对数据库中的任务有效性。
//
// 本包实现所属上下文声明的端口，序列化和 Redis 命令留在外层；改变 key 需考虑已存在的数据。
package media

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// progressClearTimeout 取消任务后清理进度快照的最长等待。
const progressClearTimeout = 2 * time.Second

// ProgressCache 转码进度的 Redis 适配器，同时实现读端口与写端口。
type ProgressCache struct {
	redis  *redis.Client
	logger *zap.Logger
}

var (
	_ domain.ProgressStore  = (*ProgressCache)(nil)
	_ domain.ProgressReader = (*ProgressCache)(nil)
)

// NewProgressCache 构造转码进度缓存适配器。
func NewProgressCache(redisClient *redis.Client, logger *zap.Logger) *ProgressCache {
	return &ProgressCache{redis: redisClient, logger: logger}
}

// Write 写入进度快照，维护分P 集合并广播变更（最佳努力）。
func (c *ProgressCache) Write(ctx context.Context, snapshot *domain.TranscodeProgress, ttl time.Duration) error {
	if c == nil || c.redis == nil || snapshot == nil || snapshot.VideoID == 0 {
		return nil
	}

	snapshot.Normalize()

	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	if err := c.redis.Set(ctx, domain.TranscodeProgressKey(snapshot.VideoID, snapshot.PartID), raw, ttl).Err(); err != nil {
		c.logger.Warn("write transcode progress failed",
			zap.Uint("videoId", snapshot.VideoID),
			zap.Uint("partId", snapshot.PartID),
			zap.Error(err),
		)

		return err
	}

	// 显式维护每个视频的分P 集合，避免某个分P 的进度覆盖另一个分P。
	if snapshot.PartID > 0 {
		partsKey := domain.TranscodeProgressPartsKey(snapshot.VideoID)

		pipe := c.redis.Pipeline()
		pipe.HSet(ctx, partsKey, domain.PartFieldKey(snapshot.PartID), raw)
		pipe.Expire(ctx, partsKey, ttl)
		_, _ = pipe.Exec(ctx)
	}

	// Pub/Sub 为最佳努力；重连后仍以 Redis 快照为准。
	_ = c.redis.Publish(ctx, constants.TranscodeProgressChannel, raw).Err()

	return nil
}

// Clear 删除进度快照与分P 集合中的对应字段（视频被删除时调用）。
func (c *ProgressCache) Clear(ctx context.Context, videoID, partID uint) {
	if c == nil || c.redis == nil || videoID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), progressClearTimeout)
	defer cancel()

	pipe := c.redis.Pipeline()
	pipe.Del(cleanupCtx, domain.TranscodeProgressKey(videoID, partID))

	if partID > 0 {
		pipe.HDel(cleanupCtx, domain.TranscodeProgressPartsKey(videoID), domain.PartFieldKey(partID))
	} else {
		pipe.Del(cleanupCtx, domain.TranscodeProgressPartsKey(videoID))
	}

	if _, err := pipe.Exec(cleanupCtx); err != nil && !errors.Is(err, redis.Nil) {
		c.logger.Warn("clear canceled transcode progress failed",
			zap.Uint("videoId", videoID),
			zap.Uint("partId", partID),
			zap.Error(err),
		)
	}
}

// Get 读取单条进度；未命中时返回 (nil, nil)。
func (c *ProgressCache) Get(ctx context.Context, videoID, partID uint) (*domain.TranscodeProgress, error) {
	if c == nil || c.redis == nil || videoID == 0 {
		return nil, nil
	}

	raw, err := c.redis.Get(ctx, domain.TranscodeProgressKey(videoID, partID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, err
	}

	var p domain.TranscodeProgress
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// ByVideoIDs 批量读取视频维度进度：存在分P 进度时展开为各分P，否则取视频级 key。
func (c *ProgressCache) ByVideoIDs(ctx context.Context, videoIDs []uint) ([]domain.TranscodeProgress, error) {
	if c == nil || c.redis == nil || len(videoIDs) == 0 {
		return []domain.TranscodeProgress{}, nil
	}

	seen := make(map[uint]struct{}, len(videoIDs))
	ids := make([]uint, 0, len(videoIDs))

	for _, id := range videoIDs {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return []domain.TranscodeProgress{}, nil
	}

	pipe := c.redis.Pipeline()
	videoCmds := make(map[uint]*redis.StringCmd, len(ids))
	partCmds := make(map[uint]*redis.StringSliceCmd, len(ids))

	for _, id := range ids {
		videoCmds[id] = pipe.Get(ctx, domain.TranscodeProgressKey(id, 0))
		partCmds[id] = pipe.HVals(ctx, domain.TranscodeProgressPartsKey(id))
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	out := make([]domain.TranscodeProgress, 0, len(ids))

	for _, id := range ids {
		partValues := partCmds[id].Val()
		if len(partValues) > 0 {
			for _, raw := range partValues {
				if p, ok := decodeTranscodeProgress(raw, c.logger); ok {
					out = append(out, p)
				}
			}

			continue
		}

		if p, ok := decodeTranscodeProgress(videoCmds[id].Val(), c.logger); ok {
			out = append(out, p)
		}
	}

	return out, nil
}

// ByPairs 按显式 (videoId, partId) 组合批量读取进度。
func (c *ProgressCache) ByPairs(ctx context.Context, pairs [][2]uint) ([]domain.TranscodeProgress, error) {
	if c == nil || c.redis == nil || len(pairs) == 0 {
		return []domain.TranscodeProgress{}, nil
	}

	keys := make([]string, 0, len(pairs))

	for _, pair := range pairs {
		if pair[0] == 0 {
			continue
		}

		keys = append(keys, domain.TranscodeProgressKey(pair[0], pair[1]))
	}

	if len(keys) == 0 {
		return []domain.TranscodeProgress{}, nil
	}

	vals, err := c.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	out := make([]domain.TranscodeProgress, 0, len(vals))

	for _, v := range vals {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}

		var p domain.TranscodeProgress
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			continue
		}

		out = append(out, p)
	}

	return out, nil
}

// Subscribe 订阅进度变更；返回的句柄由调用方负责 Close。
func (c *ProgressCache) Subscribe(ctx context.Context) (domain.ProgressSubscription, error) {
	if c == nil || c.redis == nil {
		return nil, errors.New("redis client unavailable")
	}

	pubsub := c.redis.Subscribe(ctx, constants.TranscodeProgressChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()

		return nil, err
	}

	return newProgressSubscription(ctx, pubsub, c.logger), nil
}

// progressSubscription 把 Redis Pub/Sub 消息解码成领域进度快照。
type progressSubscription struct {
	ctx       context.Context
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
	pubsub    *redis.PubSub
	events    chan domain.TranscodeProgress
	logger    *zap.Logger
}

var _ domain.ProgressSubscription = (*progressSubscription)(nil)

func newProgressSubscription(ctx context.Context, pubsub *redis.PubSub, logger *zap.Logger) *progressSubscription {
	s := &progressSubscription{
		ctx:    ctx,
		done:   make(chan struct{}),
		pubsub: pubsub,
		events: make(chan domain.TranscodeProgress, 32),
		logger: logger,
	}

	go s.pump()

	return s
}

func (s *progressSubscription) pump() {
	defer close(s.events)
	defer s.Close()
	input := s.pubsub.Channel()
	for {
		select {
		case <-s.done:
			return
		case <-s.ctx.Done():
			return
		case message, ok := <-input:
			if !ok {
				return
			}
			p, ok := decodeTranscodeProgress(message.Payload, s.logger)
			if !ok {
				continue
			}
			select {
			case <-s.done:
				return
			case <-s.ctx.Done():
				return
			case s.events <- p:
			}
		}
	}
}

// Events 返回解码后的进度快照通道。
func (s *progressSubscription) Events() <-chan domain.TranscodeProgress {
	return s.events
}

// Close 关闭底层订阅（pump goroutine 随之退出）。
func (s *progressSubscription) Close() error {
	s.closeOnce.Do(func() { close(s.done); s.closeErr = s.pubsub.Close() })
	return s.closeErr
}

func decodeTranscodeProgress(raw string, logger *zap.Logger) (domain.TranscodeProgress, bool) {
	if strings.TrimSpace(raw) == "" {
		return domain.TranscodeProgress{}, false
	}

	var p domain.TranscodeProgress
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		logger.Debug("skip invalid transcode progress json", zap.Error(err))

		return domain.TranscodeProgress{}, false
	}

	return p, true
}
