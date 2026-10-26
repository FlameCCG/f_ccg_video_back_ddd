// Package scheduler 将 cron 触发器接到平台维护和上传清理用例。
//
// cron.go/jobs.go 负责调度生命周期与触发；其余文件适配点赞队列、视频统计、热榜和到期封禁的数据访问。
// 业务计算留在 application/platform 与 domain/platform，上传清理经 UploadCleanupJob 端口调用 video 用例。
// Redis 到数据库的回写跨越两个系统，消费确认与版本比较决定失败后的恢复行为，不能按数据库事务理解整个任务。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package scheduler

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

const (
	// cronTimezone 调度时区（与原实现一致）。
	cronTimezone = "Asia/Tokyo"
	// cronStopTimeout 停止调度时等待在途任务的上限（与原实现的 5s 一致）。
	cronStopTimeout = 5 * time.Second
)

// cron 表达式（六段，含秒位；与原 cron_service.Cron 逐字一致）。
const (
	// SpecSyncCommentLike 每 30 秒刷写评论点赞数据。
	SpecSyncCommentLike = "*/30 * * * * ?"
	// SpecSyncVideoStat 每 5 分钟同步一次视频统计。
	SpecSyncVideoStat = "0 */5 * * * ?"
	// SpecDecayVideoHotScore 每 10 分钟衰减一次热门榜。
	SpecDecayVideoHotScore = "0 */10 * * * ?"
	// SpecAutoUnbanUsers 每 5 分钟检查一次自动解封。
	SpecAutoUnbanUsers = "0 */5 * * * ?"
	SpecCleanupUploads = "0 */5 * * * ?"
)

// cronJob 一条定时任务注册项。
type cronJob struct {
	// name 任务名，用于注册失败日志（与原实现的函数名一致）。
	name string
	// spec 六段 cron 表达式。
	spec string
	// run 任务执行体。
	run func(context.Context)
}

// CronComponent 定时任务调度组件。
//
// 生命周期方法 Start / Stop 与 App 的其它后台组件一致；
// Stop 幂等，且会尽量等待在途任务结束。
type CronComponent struct {
	commentLike   CommentLikeSyncJob
	videoStat     VideoStatSyncJob
	hotRank       HotRankDecayJob
	autoUnban     AutoUnbanJob
	uploadCleanup UploadCleanupJob
	logger        *zap.Logger

	mu      sync.Mutex
	crontab *cron.Cron
}

// NewCronComponent 构造定时任务调度组件。
//
// 既有四个任务由 application/platform 实现，上传清理由 application/video 实现。
func NewCronComponent(
	commentLike CommentLikeSyncJob,
	videoStat VideoStatSyncJob,
	hotRank HotRankDecayJob,
	autoUnban AutoUnbanJob,
	uploadCleanup UploadCleanupJob,
	logger *zap.Logger,
) *CronComponent {
	return &CronComponent{
		commentLike:   commentLike,
		videoStat:     videoStat,
		hotRank:       hotRank,
		autoUnban:     autoUnban,
		uploadCleanup: uploadCleanup,
		logger:        logger,
	}
}

// Start 注册并启动全部定时任务。
//
// 单个任务注册失败只记录错误日志、不阻断其余任务，因此始终返回 nil；
// 返回 error 是为了与 App 的后台组件启动签名保持一致。
// 重复调用在已启动状态下直接返回，避免遗留一个无人持有的调度器。
func (c *CronComponent) Start(_ context.Context) error {
	return c.startJobs(c.jobs())
}

// jobs 保留原 cron_service.Cron 的四个任务顺序，并追加上传清理。
func (c *CronComponent) jobs() []cronJob {
	// 定时任务是调度 root 场景，没有请求上下文，每次触发从 Background 派生。
	jobs := []cronJob{
		{
			name: "SyncCommentLike",
			spec: SpecSyncCommentLike,
			run:  func(ctx context.Context) { c.commentLike.Run(ctx) },
		},
		{
			name: "SyncVideoStat",
			spec: SpecSyncVideoStat,
			run:  func(ctx context.Context) { c.videoStat.Run(ctx) },
		},
		{
			name: "DecayVideoHotScore",
			spec: SpecDecayVideoHotScore,
			run:  func(ctx context.Context) { c.hotRank.Run(ctx) },
		},
		{
			name: "AutoUnbanUsers",
			spec: SpecAutoUnbanUsers,
			run:  func(ctx context.Context) { c.autoUnban.Run(ctx) },
		},
	}
	if c.uploadCleanup != nil {
		jobs = append(jobs, cronJob{
			name: "CleanupUploads", spec: SpecCleanupUploads,
			run: func(parent context.Context) {
				ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
				defer cancel()

				c.uploadCleanup.Run(ctx)
			},
		})
	}

	return jobs
}

// startJobs 用给定注册项构建并启动调度器。
func (c *CronComponent) startJobs(jobs []cronJob) error {
	c.mu.Lock()
	if c.crontab != nil {
		c.mu.Unlock()
		c.logger.Warn("定时任务已在运行，忽略重复启动", zap.String("event", "scheduler.cron.start_jobs.degraded"), zap.String("operation", "scheduler.cron.start_jobs"))

		return nil
	}
	c.mu.Unlock()

	crontab := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(c.cronLocation()),
		cron.WithChain(
			cron.SkipIfStillRunning(cronLogger{logger: c.logger}),
			cron.Recover(cronLogger{logger: c.logger}),
		),
	)

	for _, job := range jobs {
		if _, err := crontab.AddFunc(job.spec, func() {
			ctx := logmeta.With(context.Background(), logmeta.Values{TaskID: logmeta.NewID(), AttemptID: logmeta.NewID()})
			logger := logctx.With(ctx, c.logger)
			start := time.Now()

			logger.Info("scheduler.task.started", zap.String("event", "scheduler.task.started"), zap.String("operation", job.name))

			defer func() {
				if value := recover(); value != nil {
					logger.Error("scheduler.task.failed", zap.String("event", "scheduler.task.failed"), zap.String("operation", job.name), zap.String("error", logsafe.Text(fmt.Sprint(value), logsafe.ErrorLimit)), zap.String("stack", string(debug.Stack())))
					return
				}

				logger.Info("scheduler.task.finished", zap.String("event", "scheduler.task.finished"), zap.String("operation", job.name), zap.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)))
			}()

			job.run(ctx)
		}); err != nil {
			c.logger.Error("register "+job.name+" cron failed", zap.String("event", "scheduler.cron.start_jobs.failed"), zap.String("operation", "scheduler.cron.start_jobs"), zap.Error(err))
		}
	}

	crontab.Start()

	c.mu.Lock()
	c.crontab = crontab
	c.mu.Unlock()

	c.logger.Info("定时任务启动成功!", zap.String("event", "scheduler.cron.start_jobs.diagnostic"), zap.String("operation", "scheduler.cron.start_jobs"))

	return nil
}

// Stop 停止定时任务调度，并尽量等待在途任务结束（最多 5s）。
//
// 等价原 cron_service.Stop：幂等，等待窗口固定为 5s、不受传入 context 影响，
// 保证与原优雅退出时序完全一致。
func (c *CronComponent) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	crontab := c.crontab
	c.crontab = nil
	c.mu.Unlock()

	if crontab == nil {
		return nil
	}

	stopCtx := crontab.Stop()
	select {
	case <-stopCtx.Done():
		logctx.With(ctx, c.logger).Info("定时任务已停止", zap.String("event", "scheduler.cron.stop.diagnostic"), zap.String("operation", "scheduler.cron.stop"))
	case <-time.After(cronStopTimeout):
		logctx.With(ctx, c.logger).Warn("定时任务停止超时，继续退出", zap.String("event", "scheduler.cron.stop.degraded"), zap.String("operation", "scheduler.cron.stop"))
	}

	return nil
}

// cronLocation 解析调度时区。
//
// 原实现忽略 LoadLocation 的错误，一旦运行环境缺少 tzdata 就会得到 nil *time.Location，
// 使 cron 在计算下次触发点时 panic（time.Time.In(nil)）。
// 这里回落到本地时区并告警；tzdata 可用时（正常部署）行为与原实现完全一致。
func (c *CronComponent) cronLocation() *time.Location {
	timezone, err := time.LoadLocation(cronTimezone)
	if err != nil || timezone == nil {
		c.logger.Warn("load cron timezone failed, fallback to local", zap.String("event", "scheduler.cron.cron_location.degraded"), zap.String("operation", "scheduler.cron.cron_location"),
			zap.String("timezone", cronTimezone), zap.Error(err))

		return time.Local
	}

	return timezone
}

// cronLogger routes the scheduler library's diagnostics through its injected logger.
type cronLogger struct{ logger *zap.Logger }

func (l cronLogger) Info(msg string, _ ...any) {
	logctx.With(context.Background(), l.logger).Debug(msg, zap.String("event", "scheduler.diagnostic"))
}

func (l cronLogger) Error(err error, msg string, _ ...any) {
	logctx.With(context.Background(), l.logger).Error(msg, zap.String("event", "scheduler.diagnostic.failed"), zap.Error(err))
}
