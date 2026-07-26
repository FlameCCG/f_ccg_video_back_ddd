// Package scheduler 提供 cron 定时任务调度组件与其数据适配器（原 service/cron_service）。
//
// 原实现是包级单例（cron_service.Cron / cron_service.Stop），
// 新架构改为可注入组件 CronComponent，由 bootstrap 创建唯一实例并纳入 App 生命周期。
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
	commentLike CommentLikeSyncJob
	videoStat   VideoStatSyncJob
	hotRank     HotRankDecayJob
	autoUnban   AutoUnbanJob

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
) *CronComponent {
	return &CronComponent{
		commentLike: commentLike,
		videoStat:   videoStat,
		hotRank:     hotRank,
		autoUnban:   autoUnban,
	}
}

// Start 注册并启动全部定时任务（等价原 cron_service.Cron）。
//
// 单个任务注册失败只记录错误日志、不阻断其余任务，因此始终返回 nil；
// 返回 error 是为了与 App 的后台组件启动签名保持一致。
// 重复调用在已启动状态下直接返回，避免遗留一个无人持有的调度器。
func (c *CronComponent) Start(_ context.Context) error {
	return c.startJobs(c.jobs())
}

// jobs 返回全部注册项。顺序即原 cron_service.Cron 中的 AddFunc 顺序。
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
		zap.L().Warn("定时任务已在运行，忽略重复启动")

		return nil
	}
	c.mu.Unlock()

	crontab := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(cronLocation()),
		cron.WithChain(
			cron.SkipIfStillRunning(cron.DefaultLogger),
			cron.Recover(cron.DefaultLogger),
		),
	)

	for _, job := range jobs {
		if _, err := crontab.AddFunc(job.spec, job.run); err != nil {
			zap.L().Error("register "+job.name+" cron failed", zap.Error(err))
		}
	}

	crontab.Start()

	c.mu.Lock()
	c.crontab = crontab
	c.mu.Unlock()

	zap.L().Info("定时任务启动成功!")

	return nil
}

// Stop 停止定时任务调度，并尽量等待在途任务结束（最多 5s）。
//
// 等价原 cron_service.Stop：幂等，等待窗口固定为 5s、不受传入 context 影响，
// 保证与原优雅退出时序完全一致。
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
		zap.L().Info("定时任务已停止")
	case <-time.After(cronStopTimeout):
		zap.L().Warn("定时任务停止超时，继续退出")
	}

	return nil
}

// cronLocation 解析调度时区。
//
// 原实现忽略 LoadLocation 的错误，一旦运行环境缺少 tzdata 就会得到 nil *time.Location，
// 使 cron 在计算下次触发点时 panic（time.Time.In(nil)）。
// 这里回落到本地时区并告警；tzdata 可用时（正常部署）行为与原实现完全一致。
func cronLocation() *time.Location {
	timezone, err := time.LoadLocation(cronTimezone)
	if err != nil || timezone == nil {
		zap.L().Warn("load cron timezone failed, fallback to local",
			zap.String("timezone", cronTimezone), zap.Error(err))

		return time.Local
	}

	return timezone
}
