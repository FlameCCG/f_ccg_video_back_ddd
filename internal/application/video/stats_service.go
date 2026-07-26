package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

const (
	applyVideoStatsDeltaMaxRetry   = 2
	applyVideoStatsDeltaRetryDelay = 20 * time.Millisecond
)

// StatsService 视频统计计数用例服务（原 service/video_service/stats_cache.go）。
//
// 权威计数在 Redis（Lua 原子增量 + dirty 集合 + version），Redis 不可用时退回 MySQL。
// 该服务同时是 video 上下文对外暴露的计数入口（评论数由 comment 上下文调用）。
type StatsService struct {
	videos   domain.VideoRepository
	cache    StatsCache
	hotRank  HotRankCache
	authors  *AuthorService
	creators CreatorAnalytics
}

// NewStatsService 构造视频统计用例服务。
func NewStatsService(
	videos domain.VideoRepository,
	cache StatsCache,
	hotRank HotRankCache,
	authors *AuthorService,
	creators CreatorAnalytics,
) *StatsService {
	return &StatsService{
		videos:   videos,
		cache:    cache,
		hotRank:  hotRank,
		authors:  authors,
		creators: creators,
	}
}

// Load 读取视频统计：优先 Redis 权威计数，未命中回源 DB 并 SetNX 回填。
func (s *StatsService) Load(ctx context.Context, videoID uint) (*domain.Stats, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if stats, ok := s.cache.Load(ctx, videoID); ok && stats != nil {
		return stats, nil
	}

	stats, err := s.videos.PublishedStats(ctx, videoID)
	if err != nil {
		return nil, err
	}

	s.cache.Seed(ctx, videoID, stats)

	return stats, nil
}

// LoadBatch 批量读取 Redis 权威计数（缺失项不出现在结果里）。
func (s *StatsService) LoadBatch(ctx context.Context, videoIDs []uint) map[uint]*domain.Stats {
	if len(videoIDs) == 0 {
		return nil
	}

	return s.cache.LoadBatch(ctx, videoIDs)
}

// ApplyDelta 应用统计增量。
//
// Redis 完全不可用时才直接写 DB；有 Redis 时 Redis 是权威源。
// 与原 applyVideoStatsDelta 的重试、降级与「禁止 clear / 禁止 DB 覆盖」策略逐行一致。
func (s *StatsService) ApplyDelta(ctx context.Context, videoID uint, delta domain.StatsDelta) (*domain.Stats, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if delta.IsZero() {
		return s.Load(ctx, videoID)
	}

	if !s.cache.Available() {
		return s.applyDeltaToDB(ctx, videoID, delta)
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	var lastErr error

	for attempt := 0; attempt <= applyVideoStatsDeltaMaxRetry; attempt++ {
		if attempt > 0 {
			time.Sleep(applyVideoStatsDeltaRetryDelay)
		}

		basePayload, err := s.loadBasePayload(ctx, videoID)
		if err != nil {
			lastErr = err
			// 读 base 失败可能是瞬时 Redis 问题，优先重试而不是用 DB 覆盖权威值。
			if s.cache.IsTransientError(err) {
				continue
			}
			// 无法安全写 Redis：落 DB 增量并失效 Redis，避免陈旧 key 长期遮挡/脏同步盖回 DB。
			stats, dbErr := s.applyDeltaToDB(ctx, videoID, delta)
			if dbErr != nil {
				return nil, fmt.Errorf("stats base load failed: %w; db fallback: %v", err, dbErr)
			}

			zap.L().Warn("apply video stats delta: base load failed, wrote DB and invalidated redis",
				zap.Uint("videoId", videoID), zap.Error(err))
			s.cache.Clear(ctx, videoID)

			return stats, nil
		}

		stats, err := s.cache.ApplyDelta(ctx, videoID, delta, basePayload)
		if err == nil {
			if stats != nil {
				return stats, nil
			}

			// Lua 返回值无法解析：视为一次可重试失败（原实现此处误包装了 nil error，已修正）。
			lastErr = errStatsLuaResultUnparsable

			continue
		}

		lastErr = err
		if s.cache.IsTransientError(err) {
			continue
		}

		break
	}

	// 禁止 Lua 失败后 clear Redis、禁止用 DB 增量回填覆盖权威计数。
	// 仅记录错误，让调用方感知；权威值仍以 Redis 为准（若 key 仍在）。
	zap.L().Error("apply video stats delta via redis failed; leave redis intact (no clear, no db overwrite)",
		zap.Uint("videoId", videoID), zap.Error(lastErr))

	// 尽量返回当前 Redis 值，便于接口继续响应。
	if stats, ok := s.cache.Load(ctx, videoID); ok && stats != nil {
		return stats, lastErr
	}

	return nil, lastErr
}

// ApplyCommittedDelta 在 MySQL 业务已提交后更新展示计数。
//
// Redis 成功则走权威增量；失败则写 DB 增量并失效 Redis，使后续读从 DB 重建，
// 避免「DB 已加、Redis 仍旧」被 dirty 同步盖回。
// 用于投币/三连等不可因计数失败回滚账本的路径。
func (s *StatsService) ApplyCommittedDelta(
	ctx context.Context,
	videoID uint,
	delta domain.StatsDelta,
) (*domain.Stats, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if delta.IsZero() {
		return s.Load(ctx, videoID)
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	stats, err := s.ApplyDelta(ctx, videoID, delta)
	if err == nil && stats != nil {
		return stats, nil
	}

	dbStats, dbErr := s.applyDeltaToDB(ctx, videoID, delta)
	if dbErr != nil {
		zap.L().Error("committed video stats: redis and db both failed",
			zap.Uint("videoId", videoID),
			zap.Error(err),
			zap.NamedError("dbErr", dbErr),
		)

		// 尽量失效陈旧 Redis，降低长期错误展示概率。
		s.cache.Clear(ctx, videoID)

		if err != nil {
			return nil, fmt.Errorf("stats redis: %w; db: %v", err, dbErr)
		}

		return nil, dbErr
	}

	zap.L().Error("committed video stats: redis failed, applied db delta and invalidated redis",
		zap.Uint("videoId", videoID),
		zap.Error(err),
	)
	s.cache.Clear(ctx, videoID)

	// 失效后回源 DB 并 SetNX 回填，保证后续读到新值。
	if reloaded, loadErr := s.Load(ctx, videoID); loadErr == nil && reloaded != nil {
		return reloaded, nil
	}

	return dbStats, nil
}

// ApplyDeltaIfCached 仅当 Redis stats key 已存在时应用增量。
//
// 用于「DB 已是正确值、缓存只做热路径 patch」场景（如删收藏夹后 DB 已减）。
func (s *StatsService) ApplyDeltaIfCached(
	ctx context.Context,
	videoID uint,
	delta domain.StatsDelta,
) (*domain.Stats, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if delta.IsZero() {
		return s.Load(ctx, videoID)
	}

	if !s.cache.Available() {
		return s.Load(ctx, videoID)
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	stats, existed, err := s.cache.ApplyDeltaIfExists(ctx, videoID, delta)
	if err != nil {
		zap.L().Warn("apply video stats delta if-cached failed", zap.Uint("videoId", videoID), zap.Error(err))

		return s.Load(ctx, videoID)
	}

	if !existed {
		// 冷缓存：DB 已是权威，直接回源 DB 读，不 delta。
		return s.videos.PublishedStats(ctx, videoID)
	}

	if stats == nil {
		return s.Load(ctx, videoID)
	}

	return stats, nil
}

// InvalidateStatsCache 仅在确认 stats 已安全落库后调用。普通元数据更新禁止走此路径。
func (s *StatsService) InvalidateStatsCache(ctx context.Context, videoIDs ...uint) {
	s.cache.Clear(ctx, videoIDs...)
}

// RecordHotScoreDelta 累加视频热度分。
func (s *StatsService) RecordHotScoreDelta(ctx context.Context, videoID uint, weight float64, delta int64) {
	if videoID == 0 {
		return
	}

	score := domain.HotScoreDelta(weight, delta)
	if score == 0 {
		return
	}

	if !s.hotRank.Available() {
		return
	}

	s.hotRank.IncrBy(ctx, videoID, score)
}

// RecordHotScoreForComment 评论数变化时累加热度分（供 comment 上下文调用）。
func (s *StatsService) RecordHotScoreForComment(ctx context.Context, videoID uint, delta int64) {
	s.RecordHotScoreDelta(ctx, videoID, domain.HotScoreCommentWeight, delta)
}

// UpdateCommentCount 更新视频评论数缓存并累加作者创作中心指标（供 comment 上下文调用）。
func (s *StatsService) UpdateCommentCount(ctx context.Context, videoID uint, delta int64) {
	if _, err := s.ApplyDelta(ctx, videoID, domain.StatsDelta{CommentCount: delta}); err != nil {
		// 仅记录错误，不 clear redis，避免丢失并发增量。
		zap.L().Warn("update comment count failed",
			zap.Error(err),
			zap.Uint("videoId", videoID),
			zap.Int64("delta", delta),
		)

		return
	}

	if authorID, err := s.authors.AuthorID(ctx, videoID); err == nil {
		_ = s.creators.IncrComments(ctx, authorID, delta)
	}
}

// loadBasePayload 读取 Lua 增量所需的基线 payload：
// Redis 已有则用之，未命中则回源 DB 并序列化。
func (s *StatsService) loadBasePayload(ctx context.Context, videoID uint) (string, error) {
	raw, ok, err := s.cache.RawPayload(ctx, videoID)
	if err != nil {
		return "", err
	}

	if ok {
		return raw, nil
	}

	stats, err := s.videos.PublishedStats(ctx, videoID)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(stats)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *StatsService) applyDeltaToDB(ctx context.Context, videoID uint, delta domain.StatsDelta) (*domain.Stats, error) {
	if delta.IsZero() {
		return s.videos.PublishedStats(ctx, videoID)
	}

	if err := s.videos.ApplyStatsDelta(ctx, videoID, delta); err != nil {
		return nil, err
	}

	return s.videos.PublishedStats(ctx, videoID)
}

// errStatsLuaResultUnparsable Lua 返回值无法解析为 Stats。
var errStatsLuaResultUnparsable = errors.New("unmarshal stats lua result")
