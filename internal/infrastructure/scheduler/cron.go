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
	run func()
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
// 单个任务注册失败只记录错误日志、不阻断其余任务。
// 重复调用在已启动状态下直接返回，避免遗留一个无人持有的调度器。
func (c *CronComponent) Start(_ context.Context) {
	c.startJobs(c.jobs())
}

// jobs 保留原 cron_service.Cron 的四个任务顺序，并追加上传清理。
func (c *CronComponent) jobs() []cronJob {
	// 定时任务是调度 root 场景，没有请求上下文，每次触发从 Background 派生。
	jobs := []cronJob{
		{
			name: "SyncCommentLike",
			spec: SpecSyncCommentLike,
			run:  func() { c.commentLike.Run(context.Background()) },
		},
		{
			name: "SyncVideoStat",
			spec: SpecSyncVideoStat,
			run:  func() { c.videoStat.Run(context.Background()) },
		},
		{
			name: "DecayVideoHotScore",
			spec: SpecDecayVideoHotScore,
			run:  func() { c.hotRank.Run(context.Background()) },
		},
		{
			name: "AutoUnbanUsers",
			spec: SpecAutoUnbanUsers,
			run:  func() { c.autoUnban.Run(context.Background()) },
		},
	}
	if c.uploadCleanup != nil {
		jobs = append(jobs, cronJob{
			name: "CleanupUploads", spec: SpecCleanupUploads,
			run: func() {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
				defer cancel()
				c.uploadCleanup.Run(ctx)
			},
		})
	}
	return jobs
}

// startJobs 用给定注册项构建并启动调度器。
func (c *CronComponent) startJobs(jobs []cronJob) {
	c.mu.Lock()
	if c.crontab != nil {
		c.mu.Unlock()
		c.logger.Warn("定时任务已在运行，忽略重复启动")

		return
	}
	c.mu.Unlock()

	crontab := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(c.cronLocation()),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)

	for _, job := range jobs {
		if _, err := crontab.AddFunc(job.spec, job.run); err != nil {
			c.logger.Error("register "+job.name+" cron failed", zap.Error(err))
		}
	}

	crontab.Start()

	c.mu.Lock()
	c.crontab = crontab
	c.mu.Unlock()

	c.logger.Info("定时任务启动成功!")
}

// Stop 停止定时任务调度，并尽量等待在途任务结束（最多 5s）。
//
// 等价原 cron_service.Stop：幂等，等待窗口固定为 5s、不受传入 context 影响，
// 保证与原优雅退出时序完全一致。
func (c *CronComponent) Stop(_ context.Context) {
	if c == nil {
		return
	}

	c.mu.Lock()
	crontab := c.crontab
	c.crontab = nil
	c.mu.Unlock()

	if crontab == nil {
		return
	}

	stopCtx := crontab.Stop()
	select {
	case <-stopCtx.Done():
		c.logger.Info("定时任务已停止")
	case <-time.After(cronStopTimeout):
		c.logger.Warn("定时任务停止超时，继续退出")
	}
}

// cronLocation 解析调度时区。
//
// 原实现忽略 LoadLocation 的错误，一旦运行环境缺少 tzdata 就会得到 nil *time.Location，
// 使 cron 在计算下次触发点时 panic（time.Time.In(nil)）。
// 这里回落到本地时区并告警；tzdata 可用时（正常部署）行为与原实现完全一致。
func (c *CronComponent) cronLocation() *time.Location {
	timezone, err := time.LoadLocation(cronTimezone)
	if err != nil || timezone == nil {
		c.logger.Warn("load cron timezone failed, fallback to local",
			zap.String("timezone", cronTimezone), zap.Error(err))

		return time.Local
	}

	return timezone
}
