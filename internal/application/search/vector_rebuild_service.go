package search

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

const (
	vectorRebuildPageSize       = 200
	vectorRebuildDefaultWorkers = 4
	vectorRebuildMaxWorkers     = 16
	vectorRebuildTimeout        = 6 * time.Hour
)

// RebuildOptions 控制向量索引全量回填。
type RebuildOptions struct {
	Recreate bool
	Workers  int
}

// RebuildStatus 是全量回填任务的可观测快照。
type RebuildStatus struct {
	Running    bool      `json:"running"`
	Total      int64     `json:"total"`
	Succeeded  int64     `json:"succeeded"`
	Failed     int64     `json:"failed"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	LastError  string    `json:"lastError,omitempty"`
}

// VideoVectorRebuildService 管理进程内唯一的视频向量全量回填任务。
type VideoVectorRebuildService struct {
	vectors *VideoVectorService
	source  VideoVectorRebuildSource
	index   VideoVectorIndexLifecycle
	tasks   *ctxutil.DetachedTaskGroup
	logger  *zap.Logger

	mu         sync.Mutex
	running    bool
	startedAt  time.Time
	finishedAt time.Time
	lastError  string
	total      atomic.Int64
	succeeded  atomic.Int64
	failed     atomic.Int64
}

// NewVideoVectorRebuildService 构造受应用生命周期管理的向量回填服务。
func NewVideoVectorRebuildService(
	vectors *VideoVectorService,
	source VideoVectorRebuildSource,
	index VideoVectorIndexLifecycle,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *VideoVectorRebuildService {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &VideoVectorRebuildService{
		vectors: vectors,
		source:  source,
		index:   index,
		tasks:   tasks,
		logger:  logger,
	}
}

// StartRebuild 异步启动全量回填；已有任务在运行时返回 (false, nil)。
func (s *VideoVectorRebuildService) StartRebuild(options RebuildOptions) (bool, error) {
	if s == nil || s.vectors == nil || s.source == nil || s.index == nil || s.tasks == nil {
		return false, errors.New("数据库或 ES 未初始化")
	}

	if !s.source.Available() || !s.index.Available() {
		return false, errors.New("数据库或 ES 未初始化")
	}

	cfg, err := s.vectors.vectorConfig()
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()

		return false, nil
	}

	s.running = true
	s.startedAt = time.Now()
	s.finishedAt = time.Time{}
	s.lastError = ""
	s.total.Store(0)
	s.succeeded.Store(0)
	s.failed.Store(0)
	s.mu.Unlock()

	accepted := s.tasks.Go(context.Background(), vectorRebuildTimeout, func(ctx context.Context) {
		runErr := s.run(ctx, cfg, options)

		s.mu.Lock()
		s.running = false
		s.finishedAt = time.Now()
		if runErr != nil {
			s.lastError = runErr.Error()
		}
		s.mu.Unlock()

		s.logger.Info("video vector rebuild finished",
			zap.Int64("succeeded", s.succeeded.Load()),
			zap.Int64("failed", s.failed.Load()),
			zap.Error(runErr),
		)
	})
	if !accepted {
		s.mu.Lock()
		s.running = false
		s.finishedAt = time.Now()
		s.lastError = "应用正在停止"
		s.mu.Unlock()

		return false, errors.New("应用正在停止")
	}

	return true, nil
}

// RebuildStatus 返回当前任务快照。
func (s *VideoVectorRebuildService) RebuildStatus() RebuildStatus {
	if s == nil {
		return RebuildStatus{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return RebuildStatus{
		Running:    s.running,
		Total:      s.total.Load(),
		Succeeded:  s.succeeded.Load(),
		Failed:     s.failed.Load(),
		StartedAt:  s.startedAt,
		FinishedAt: s.finishedAt,
		LastError:  s.lastError,
	}
}

func (s *VideoVectorRebuildService) run(
	ctx context.Context,
	cfg VectorConfig,
	options RebuildOptions,
) error {
	if err := s.index.Prepare(ctx, cfg, options.Recreate); err != nil {
		return err
	}

	total, err := s.source.CountPublished(ctx)
	if err != nil {
		return fmt.Errorf("统计待重建视频失败: %w", err)
	}
	s.total.Store(total)

	workers := options.Workers
	if workers <= 0 {
		workers = vectorRebuildDefaultWorkers
	}
	workers = min(workers, vectorRebuildMaxWorkers)

	jobs := make(chan VideoVectorSource)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for source := range jobs {
				if err := s.vectors.indexVideoVectorSourceWithConfig(ctx, cfg, source); err != nil {
					s.failed.Add(1)
					s.logger.Warn("rebuild video vector failed",
						zap.Uint("videoId", source.ID),
						zap.Error(err),
					)

					continue
				}
				s.succeeded.Add(1)
			}
		}()
	}

	var cursor uint
	for {
		sources, loadErr := s.source.PublishedAfter(ctx, cursor, vectorRebuildPageSize)
		if loadErr != nil {
			close(jobs)
			wait.Wait()

			return fmt.Errorf("读取待重建视频失败: %w", loadErr)
		}
		if len(sources) == 0 {
			break
		}

		cursor = sources[len(sources)-1].ID
		for _, source := range sources {
			select {
			case jobs <- source:
			case <-ctx.Done():
				close(jobs)
				wait.Wait()

				return ctx.Err()
			}
		}
	}

	close(jobs)
	wait.Wait()

	return s.index.Refresh(ctx, cfg)
}
