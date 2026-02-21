// Package scheduler 提供 cron 调度与数据适配器。
//
// bootstrap 创建唯一 CronComponent，并纳入 App 生命周期。
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

const (
	// cronTimezone 调度时区。
	cronTimezone = "Asia/Tokyo"
	// cronStopTimeout 停止调度时等待在途任务的上限，为 5s。
	cronStopTimeout = 5 * time.Second
)

// cron 表达式使用包含秒位的六段格式。
const (
	// SpecSyncCommentLike 每 30 秒刷写评论点赞数据。
	SpecSyncCommentLike = "*/30 * * * * ?"
	// SpecSyncVideoStat 每 5 分钟同步一次视频统计。
	SpecSyncVideoStat = "0 */5 * * * ?"
	// SpecDecayVideoHotScore 每 10 分钟衰减一次热门榜。
	SpecDecayVideoHotScore = "0 */10 * * * ?"
	// SpecAutoUnbanUsers 每 5 分钟检查一次自动解封。
	SpecAutoUnbanUsers = "0 */5 * * * ?"
)

// cronJob 一条定时任务注册项。
type cronJob struct {
	// name 是任务注册失败日志中的任务名。
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
	commentLike CommentLikeSyncJob
	videoStat   VideoStatSyncJob
	hotRank     HotRankDecayJob
	autoUnban   AutoUnbanJob
	logger      *zap.Logger

	mu      sync.Mutex
	crontab *cron.Cron
}

// NewCronComponent 构造定时任务调度组件。
//
// 四个任务端口分别由 application/platform 的用例服务实现，由 bootstrap 注入。
func NewCronComponent(
	commentLike CommentLikeSyncJob,
	videoStat VideoStatSyncJob,
	hotRank HotRankDecayJob,
	autoUnban AutoUnbanJob,
	logger *zap.Logger,
) *CronComponent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CronComponent{
		commentLike: commentLike,
		videoStat:   videoStat,
		hotRank:     hotRank,
		autoUnban:   autoUnban,
		logger:      logger,
	}
}

// Start 注册并启动全部定时任务。
//
// 单项注册失败仅记录错误，其余任务继续，因此始终返回 nil；error 返回值与 App 生命周期接口统一。
// 重复调用已启动的组件时直接返回，避免遗留调度器。
func (c *CronComponent) Start(_ context.Context) error {
	return c.startJobs(c.jobs())
}

// jobs 按调度依赖顺序返回全部注册项。
func (c *CronComponent) jobs() []cronJob {
	// 定时任务是调度 root 场景，没有请求上下文，每次触发从 Background 派生。
	return []cronJob{
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
}

// startJobs 用给定注册项构建并启动调度器。
func (c *CronComponent) startJobs(jobs []cronJob) error {
	c.mu.Lock()
	if c.crontab != nil {
		c.mu.Unlock()
		c.logger.Warn("定时任务已在运行，忽略重复启动")

		return nil
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

	return nil
}

// Stop 幂等停止调度并等待在途任务。
//
// 等待窗口固定为 5s，不受传入 context 影响。
func (c *CronComponent) Stop(_ context.Context) error {
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
		c.logger.Info("定时任务已停止")
	case <-time.After(cronStopTimeout):
		c.logger.Warn("定时任务停止超时，继续退出")
	}

	return nil
}

// cronLocation 解析调度时区。
//
// 环境缺少 tzdata 或解析失败时告警并回退到本地时区，避免 nil *time.Location 触发 panic。
func (c *CronComponent) cronLocation() *time.Location {
	timezone, err := time.LoadLocation(cronTimezone)
	if err != nil || timezone == nil {
		c.logger.Warn("load cron timezone failed, fallback to local",
			zap.String("timezone", cronTimezone), zap.Error(err))

		return time.Local
	}

	return timezone
}
