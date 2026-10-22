package video

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// defaultSchedulerInterval 定时发布轮询间隔（与原 schedulerService.interval 一致）。
const defaultSchedulerInterval = 5 * time.Second

// Scheduler 视频定时发布调度器。
//
// 原实现为包级单例（videoScheduler + sync.Once + InitVideoScheduler/StopVideoScheduler）；
// 新架构改为可注入组件，由 bootstrap 挂到 App 的后台组件序列并控制启停顺序。
type Scheduler struct {
	txm         tx.Manager
	videos      domain.VideoRepository
	store       ScheduleStore
	vectors     *VectorSyncService
	cfgProvider shared.ConfigReader
	interval    time.Duration
	logger      *zap.Logger

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// NewScheduler 构造视频定时发布调度器。
func NewScheduler(
	txm tx.Manager,
	videos domain.VideoRepository,
	store ScheduleStore,
	vectors *VectorSyncService,
	cfgProvider shared.ConfigReader,
	logger *zap.Logger,
) *Scheduler {
	return &Scheduler{
		txm:         txm,
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
// 调度依赖数据库，不依赖 Redis。重复调用幂等，由 Stop 统一取消。
func (s *Scheduler) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.store.Available() {
		s.logger.Warn("schedule storage not configured, skip video scheduler")

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

// Running 报告调度器当前是否在运行。
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.running
}

// SchedulePublish 添加定时发布任务。
//
// 计划持久化后可等待服务启动或重启，不要求调度循环已经运行。
func (s *Scheduler) SchedulePublish(ctx context.Context, videoID uint, when time.Time) error {
	return s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		if video == nil {
			return domain.ErrRecordNotFound
		}
		return s.store.Add(txCtx, videoID, when)
	})
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
			if err := s.store.RecoverLegacy(ctx); err != nil {
				s.logger.Warn("recover legacy video schedules failed; will retry", zap.Error(err))
			}
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

		_, pubErr := s.ProcessDueScheduledVideo(ctx, uint(videoID))
		if pubErr != nil {
			s.logger.Error("scheduled publish failed", zap.Error(pubErr), zap.Uint("videoId", uint(videoID)))
		}
	}
}

// ProcessDueScheduledVideo 处理到期定时任务。
//
// 在事务中重新检查最新发布时间；状态变更与任务消费一起提交。
// drop=true 表示任务已在事务中消费，调用方不可在提交后再次无条件删除计划。
func (s *Scheduler) ProcessDueScheduledVideo(ctx context.Context, videoID uint) (bool, error) {
	var drop, published bool
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		if video != nil {
			score, exists, err := s.store.Score(txCtx, videoID)
			if err != nil {
				return err
			}
			if !exists {
				drop = true
				return nil
			}
			if score > float64(time.Now().Unix()) {
				return nil // 列出到期任务后已被作者改期。
			}
			switch domain.ResolveScheduledPublishOutcome(video.Status, contentReviewEnabled(s.cfgProvider)) {
			case domain.ScheduledPublishFromScheduled, domain.ScheduledPublishFromReview:
				rows, err := s.videos.UpdateStatusFrom(txCtx, videoID, video.Status, enum.VideoStatusPublished)
				if err != nil {
					return err
				}
				if rows == 0 {
					return fmt.Errorf("video %d status changed during scheduled publish", videoID)
				}
				published = true
			}
		}
		if err := s.store.Remove(txCtx, videoID); err != nil {
			return err
		}
		drop = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if published {
		s.vectors.SyncAsync(ctx, videoID, enum.VideoStatusPublished)
	}
	return drop, nil
}
