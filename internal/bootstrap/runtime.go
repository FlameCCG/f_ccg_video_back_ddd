package bootstrap

import (
	"context"

	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/mq"
	"f_ccg_video_back_ddd/internal/infrastructure/river"
	"f_ccg_video_back_ddd/internal/infrastructure/scheduler"
	chatws "f_ccg_video_back_ddd/internal/interfaces/ws/chat"
	danmuws "f_ccg_video_back_ddd/internal/interfaces/ws/danmu"
	"f_ccg_video_back_ddd/internal/interfaces/ws/wsorigin"

	"go.uber.org/zap"
)

type runtimeComponent interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type namedRuntimeComponent struct {
	name      string
	component runtimeComponent
}

// RuntimeComponents 聚合必须显式启停的后台组件。
type RuntimeComponents struct {
	logger    *zap.Logger
	river     runtimeComponent
	video     runtimeComponent
	transcode runtimeComponent
	cron      runtimeComponent
	chatHub   runtimeComponent
	danmuHub  runtimeComponent
}

// NewRuntimeComponents 把所有后台组件纳入应用生命周期，避免 Wire 因无人引用而剪枝。
//
// 构造函数只登记引用，真正开始消费、调度或接收事件要等 Start。
// 停机时按显式序列调用 Stop，不依赖垃圾回收、构造器的相反顺序或 cleanup 自动推断。
func NewRuntimeComponents(
	riverComponent *river.RiverComponent,
	videoScheduler *videoapp.Scheduler,
	transcodeConsumer *mq.TranscodeConsumerComponent,
	cronComponent *scheduler.CronComponent,
	chatHub *chatws.Hub,
	danmuHub *danmuws.Hub,
	logger *zap.Logger,
) *RuntimeComponents {
	return &RuntimeComponents{
		logger:    logger,
		river:     riverComponent,
		video:     videoScheduler,
		transcode: transcodeConsumer,
		cron:      cronComponent,
		chatHub:   chatHub,
		danmuHub:  danmuHub,
	}
}

func (c *RuntimeComponents) log() *zap.Logger {
	if c == nil || c.logger == nil {
		return zap.NewNop()
	}

	return c.logger
}

// Start 按原 core.InitCore 尾部顺序启动后台组件。
// 单项失败只记录日志并继续，保持 HTTP 仍可启动的既有语义。
func (c *RuntimeComponents) Start(ctx context.Context) {
	for _, item := range c.startOrder() {
		if item.component == nil {
			continue
		}
		if err := item.component.Start(ctx); err != nil {
			c.log().Error("start runtime component failed",
				zap.String("component", item.name),
				zap.Error(err),
			)
		}
	}
}

// Stop 按原 GracefulStop 顺序停止后台组件；WebSocket Hub 在 Redis 关闭前回收。
func (c *RuntimeComponents) Stop(ctx context.Context) {
	for _, item := range c.stopOrder() {
		if item.component == nil {
			continue
		}
		if err := item.component.Stop(ctx); err != nil {
			c.log().Warn("stop runtime component failed",
				zap.String("component", item.name),
				zap.Error(err),
			)
		}
	}
}

func (c *RuntimeComponents) startOrder() []namedRuntimeComponent {
	if c == nil {
		return nil
	}

	return []namedRuntimeComponent{
		{name: "river", component: c.river},
		{name: "video_scheduler", component: c.video},
		{name: "transcode_consumer", component: c.transcode},
		{name: "cron", component: c.cron},
		{name: "chat_websocket", component: c.chatHub},
		{name: "danmu_websocket", component: c.danmuHub},
	}
}

func (c *RuntimeComponents) stopOrder() []namedRuntimeComponent {
	if c == nil {
		return nil
	}

	return []namedRuntimeComponent{
		{name: "transcode_consumer", component: c.transcode},
		{name: "video_scheduler", component: c.video},
		{name: "river", component: c.river},
		{name: "cron", component: c.cron},
		{name: "chat_websocket", component: c.chatHub},
		{name: "danmu_websocket", component: c.danmuHub},
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
