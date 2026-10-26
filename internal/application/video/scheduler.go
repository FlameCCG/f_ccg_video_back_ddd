package video

import (
	"context"
	"fmt"
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
	statuses := []enum.VideoStatus{enum.VideoStatusScheduled}
	if !contentReviewEnabled(s.cfgProvider) {
		statuses = append(statuses, enum.VideoStatusReview)
	}
	videoIDs, err := s.store.DueVideoIDs(ctx, time.Now(), statuses)
	if err != nil {
		s.logger.Error("fetch scheduled videos failed", zap.Error(err))

		return
	}

	for _, videoID := range videoIDs {
		_, pubErr := s.ProcessDueScheduledVideo(ctx, videoID)
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
	var drop, published bool
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		video, err := s.videos.FindByIDForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		if video != nil {
			if video.Status == enum.VideoStatusPublished {
				drop = true
				return nil // 其它实例已发布，保留原计划时间，不能把历史信息当作过期任务清掉。
			}
			publishAt, err := s.store.PublishTime(txCtx, videoID)
			if err != nil {
				return err
			}
			if publishAt == nil {
				drop = true
				return nil
			}
			if publishAt.After(time.Now()) {
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
				drop = true
				return nil // 保留计划时间和定时标识供后台查看；已发布状态不会再次进入到期列表。
			case domain.ScheduledPublishKeepReview:
				return nil // 待审作品保留原计划时间，审核通过时再决定立即或定时发布。
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
