package video

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// defaultSchedulerInterval 定时发布轮询间隔。
const defaultSchedulerInterval = 5 * time.Second

// Scheduler 视频定时发布调度器，由 bootstrap 加入 App 的后台组件序列，统一管理启停顺序。
type Scheduler struct {
	videos      domain.VideoRepository
	store       ScheduleStore
	vectors     *VectorSyncService
	cfgProvider *config.Provider
	interval    time.Duration
	logger      *zap.Logger

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// NewScheduler 构造视频定时发布调度器。
func NewScheduler(
	videos domain.VideoRepository,
	store ScheduleStore,
	vectors *VectorSyncService,
	cfgProvider *config.Provider,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		videos:      videos,
		store:       store,
		vectors:     vectors,
		cfgProvider: cfgProvider,
		interval:    defaultSchedulerInterval,
		logger:      logger,
	}
}

// Start 启动调度循环。
//
// Redis 未配置时只记录日志并跳过启动；
// 重复调用是幂等的。调度循环是进程级后台 goroutine，由 Stop 统一取消。
func (s *Scheduler) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.store.Available() {
		s.logger.Warn("redis not configured, skip video scheduler")

		return nil
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	s.wg.Add(1)

	go s.loop(loopCtx)

	return nil
}

// Stop 停止调度循环并等待退出。
func (s *Scheduler) Stop(_ context.Context) error {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()

		return nil
	}

	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	s.wg.Wait()

	return nil
}

// Running 报告调度器当前是否在运行（供 ScheduleVideoPublish 校验）。
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.running
}

// SchedulePublish 添加定时发布任务。
//
// 调度器未启动时返回错误。
func (s *Scheduler) SchedulePublish(ctx context.Context, videoID uint, when time.Time) error {
	if !s.Running() {
		return domain.ErrSchedulerNotRunning
	}

	return s.store.Add(orTODO(ctx), videoID, when)
}

func (s *Scheduler) loop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.flushDue(ctx)
		}
	}
}

func (s *Scheduler) flushDue(ctx context.Context) {
	members, err := s.store.DueMembers(ctx, time.Now().Unix())
	if err != nil {
		s.logger.Error("fetch scheduled videos failed", zap.Error(err))

		return
	}

	for _, member := range members {
		videoID, convErr := strconv.ParseUint(member, 10, 64)
		if convErr != nil {
			_ = s.store.RemoveMember(ctx, member)

			continue
		}

		drop, pubErr := s.ProcessDueScheduledVideo(ctx, uint(videoID))
		if pubErr != nil {
			s.logger.Error("scheduled publish failed", zap.Error(pubErr), zap.Uint("videoId", uint(videoID)))
		}

		// 成功发布、不可自动发布（待审/终态）时移除任务；瞬时失败则保留重试。
		if drop {
			if err := s.store.RemoveMember(ctx, member); err != nil {
				s.logger.Error("remove schedule member failed", zap.Error(err), zap.Uint("videoId", uint(videoID)))
			}
		}
	}
}

// ProcessDueScheduledVideo 处理到期定时任务。
//
// drop=true 表示应从 ZSet 移除（已发布 / 待审不可自动发 / 终态 / 视频不存在）。
func (s *Scheduler) ProcessDueScheduledVideo(ctx context.Context, videoID uint) (bool, error) {
	video, err := s.videos.FindByID(ctx, videoID)
	if err != nil {
		return false, err
	}

	if video == nil {
		return true, nil
	}

	switch domain.ResolveScheduledPublishOutcome(video.Status, contentReviewEnabled(s.cfgProvider)) {
	case domain.ScheduledPublishFromScheduled:
		return s.publishIfStatus(ctx, videoID, enum.VideoStatusScheduled)
	case domain.ScheduledPublishFromReview:
		return s.publishIfStatus(ctx, videoID, enum.VideoStatusReview)
	case domain.ScheduledPublishKeepReview:
		s.logger.Info("skip auto-publish: video still in review",
			zap.Uint("videoId", videoID),
		)

		return true, nil
	default:
		return true, nil
	}
}

func (s *Scheduler) publishIfStatus(ctx context.Context, videoID uint, from enum.VideoStatus) (bool, error) {
	rows, err := s.videos.UpdateStatusFrom(ctx, videoID, from, enum.VideoStatusPublished)
	if err != nil {
		return false, err
	}

	if rows == 0 {
		// 状态被并发改写，丢弃调度避免死循环。
		return true, fmt.Errorf("video %d status unchanged from %d", videoID, from)
	}

	s.vectors.SyncAsync(ctx, videoID, enum.VideoStatusPublished)

	return true, nil
}
