package video

import (
	"context"
	"fmt"
	"sync"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// defaultSchedulerInterval 定时发布轮询间隔。
const defaultSchedulerInterval = 5 * time.Second

// Scheduler 视频定时发布调度器，由 bootstrap 加入 App 的后台组件序列，统一管理启停顺序。
type Scheduler struct {
	txm         tx.Manager
	videos      videodomain.VideoRepository
	store       videodomain.ScheduleRepository
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
	videos videodomain.VideoRepository,
	store videodomain.ScheduleRepository,
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
func (s *Scheduler) Start(_ context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	if !s.store.Available() {
		s.logger.Warn("schedule storage not configured, skip video scheduler")

		return
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	s.wg.Add(1)

	go s.loop(loopCtx)
}

// Stop 停止调度循环并等待退出。
func (s *Scheduler) Stop(_ context.Context) {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()

		return
	}

	cancel := s.cancel
	s.cancel = nil
	s.running = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	s.wg.Wait()
}

// Running 报告调度器当前是否在运行。
func (s *Scheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.running
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

// flushDue 按当前审核政策筛选一批到期候选。
// 开启审核时不扫描 Review，避免轮询替代人工审核；真正发布前仍需重新锁定并检查。
func (s *Scheduler) flushDue(ctx context.Context) {
	reviewEnabled := contentReviewEnabled(s.cfgProvider.Get())
	statuses := (videodomain.Video{}).DueScanStatuses(reviewEnabled)
	videoIDs, err := s.store.DueVideoIDs(ctx, time.Now(), statuses)
	if err != nil {
		s.logger.Error("fetch scheduled videos failed", zap.Error(err))

		return
	}

	for _, videoID := range videoIDs {
		_, pubErr := s.processDueScheduledVideo(ctx, videoID, reviewEnabled)
		if pubErr != nil {
			s.logger.Error("scheduled publish failed", zap.Error(pubErr), zap.Uint("videoId", videoID))
		}
	}
}

// ProcessDueScheduledVideo 处理到期定时任务。
//
// 初次扫描只给出候选 ID，不是领取锁；多实例可能扫描到同一稿件。
// 本方法在行锁内重新读取状态和计划，以免发布已被作者改期或被管理端修改的稿件。
//
// 在事务中重新检查最新发布时间；状态变更与任务消费一起提交。
// drop=true 表示本次任务已处理；成功发布保留原计划时间，调用方不可再无条件清除。
func (s *Scheduler) ProcessDueScheduledVideo(ctx context.Context, videoID uint) (bool, error) {
	return s.processDueScheduledVideo(ctx, videoID, contentReviewEnabled(s.cfgProvider.Get()))
}

func (s *Scheduler) processDueScheduledVideo(ctx context.Context, videoID uint, reviewEnabled bool) (bool, error) {
	var drop, published bool
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		if video.IsPublished() {
			drop = true
			return nil // 已发布稿件保留原计划时间。
		}
		var publishAt *time.Time
		var previousStatus enum.VideoStatus
		if video != nil {
			previousStatus = video.Status()
			publishAt, err = s.store.PublishTime(txCtx, videoID)
			if err != nil {
				return err
			}
		}
		decision := video.PublishWhenDue(publishAt, time.Now(), reviewEnabled)
		if decision.Published {
			rows, err := s.videos.UpdateStatusFrom(txCtx, videoID, previousStatus, video.Status())
			if err != nil {
				return err
			}
			if rows == 0 {
				return fmt.Errorf("video %d status changed during scheduled publish", videoID)
			}
		}
		if decision.RemovePlan {
			if err := s.store.Remove(txCtx, videoID); err != nil {
				return err
			}
		}
		published = decision.Published
		drop = decision.Handled
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
