package bootstrap

import (
	"context"

	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/mq"
	chatrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/river"
	"f_ccg_video_back_ddd/internal/infrastructure/scheduler"
	chatws "f_ccg_video_back_ddd/internal/interfaces/ws/chat"
	danmuws "f_ccg_video_back_ddd/internal/interfaces/ws/danmu"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"

	"go.uber.org/zap"
)

type runtimeStarter interface {
	Start(context.Context)
}

type runtimeStopper interface {
	Stop(context.Context)
}

type runtimeComponent interface {
	runtimeStarter
	runtimeStopper
}

// RuntimeComponents 聚合必须显式启停的后台组件。
type RuntimeComponents struct {
	chatIndexes runtimeStarter
	river       runtimeComponent
	video       runtimeComponent
	transcode   runtimeComponent
	cron        runtimeComponent
	chatHub     runtimeStopper
	danmuHub    runtimeStopper
}

// NewRuntimeComponents 把所有后台组件纳入应用生命周期，避免 Wire 因无人引用而剪枝。
//
// 构造函数只登记引用，真正开始消费、调度或接收事件要等 Start。
// 停机时按显式序列调用 Stop，不依赖垃圾回收、构造器的相反顺序或 cleanup 自动推断。
func NewRuntimeComponents(
	chatIndexes *chatrepo.IndexInitializer,
	riverComponent *river.RiverComponent,
	videoScheduler *videoapp.Scheduler,
	transcodeConsumer *mq.TranscodeConsumerComponent,
	cronComponent *scheduler.CronComponent,
	chatHub *chatws.Hub,
	danmuHub *danmuws.Hub,
) *RuntimeComponents {
	return &RuntimeComponents{
		chatIndexes: chatIndexes,
		river:       riverComponent,
		video:       videoScheduler,
		transcode:   transcodeConsumer,
		cron:        cronComponent,
		chatHub:     chatHub,
		danmuHub:    danmuHub,
	}
}

// Start 先初始化聊天索引，再按既有顺序启动后台组件。
// 各组件内部记录失败，保持 HTTP 仍可启动的既有语义；Hub 按连接懒启动。
func (c *RuntimeComponents) Start(ctx context.Context) {
	if c == nil {
		return
	}

	for _, component := range []runtimeStarter{c.chatIndexes, c.river, c.video, c.transcode, c.cron} {
		if component != nil {
			component.Start(ctx)
		}
	}
}

// Stop 按既有顺序停止后台组件；WebSocket Hub 在 Redis 关闭前回收。
func (c *RuntimeComponents) Stop(ctx context.Context) {
	if c == nil {
		return
	}

	for _, component := range []runtimeStopper{c.transcode, c.video, c.river, c.cron, c.chatHub, c.danmuHub} {
		if component != nil {
			component.Stop(ctx)
		}
	}
}

// NewDanmuHub 构造弹幕 Hub 并回填广播端口，显式打断 Hub ↔ DanmuService 构造环。
//
// DanmuService 先构造，Hub 随后依赖服务构造，最后将 Hub 的广播能力回填到服务。
// 这一步属于组合根装配，必须在组件 Start 和对外服务前完成；运行中不应反复更换该引用。
func NewDanmuHub(
	service *videoapp.DanmuService,
	bus danmuws.EventBus,
	origin *wsorigin.Checker,
	logger *zap.Logger,
) *danmuws.Hub {
	hub := danmuws.NewHub(service, bus, origin, logger)
	service.SetBroadcaster(hub)

	return hub
}
