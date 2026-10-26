package river

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/logctx"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	test20240101000000 = "2024-01-01 00:00:00"
	testAPng           = "a.png"
	testAlice          = "alice"
	testFCcgVideo      = "f_ccg_video"
)

// Component 是 MySQL binlog → ES 同步引擎的可注入后台组件（原 core/init_river.go）。
//
// 生命周期语义与原实现完全一致：
//   - Start 只生效一次（原 sync.Once，此处用内部 started 标记等价实现）；
//   - River 未启用时只记 "river disabled" 日志；
//   - 构造失败只记 "init river failed" 错误日志，不阻断进程启动；
//   - 启动成功后后台 goroutine 跑 r.Run()，并记 "river started"；
//   - Stop 幂等，关闭后记 "river stopped"。
type Component struct {
	riverCfg *config.River
	mysqlCfg *config.Mysql
	esCfg    *config.ES
	db       *gorm.DB
	logger   *zap.Logger

	mu      sync.Mutex
	started bool
	river   *River
}

// NewRiverComponent 构造 River 后台组件（任意配置为 nil 时 Start 只记日志不报错）。
func NewRiverComponent(
	riverCfg *config.River,
	mysqlCfg *config.Mysql,
	esCfg *config.ES,
	db *gorm.DB,
	logger *zap.Logger,
) *Component {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Component{
		riverCfg: riverCfg,
		mysqlCfg: mysqlCfg,
		esCfg:    esCfg,
		db:       db,
		logger:   logger,
	}
}

// Start 启动 binlog 同步；重复调用无副作用，启动失败不返回错误（与原 core.InitRiver 一致）。
func (c *Component) Start(ctx context.Context) error {
	c.mu.Lock()

	if c.started {
		c.mu.Unlock()

		return nil
	}

	c.started = true
	c.mu.Unlock()

	if c.riverCfg == nil || !c.riverCfg.Enable {
		logctx.With(ctx, c.logger).Info("river disabled", zap.String("event", "river.component.start.diagnostic"), zap.String("operation", "river.component.start"))

		return nil
	}

	r, err := NewRiver(c.riverCfg, c.mysqlCfg, c.esCfg, c.db, c.logger)
	if err != nil {
		logctx.With(ctx, c.logger).Error("init river failed", zap.String("event", "river.component.start.failed"), zap.String("operation", "river.component.start"), zap.Error(err))

		return nil
	}

	c.mu.Lock()
	c.river = r
	c.mu.Unlock()

	go func() {
		if err := r.Run(); err != nil {
			if r.Ctx().Err() != nil {
				return
			}

			logctx.With(ctx, c.logger).Error("river stopped", zap.String("event", "river.component.start.failed"), zap.String("operation", "river.component.start"), zap.Error(err))
		}
	}()

	logctx.With(ctx, c.logger).Info("river started", zap.String("event", "river.component.start.diagnostic"), zap.String("operation", "river.component.start"))

	return nil
}

// Stop 优雅关闭同步引擎：取消上下文、关闭 canal、强制刷盘 binlog 位点（幂等）。
func (c *Component) Stop(ctx context.Context) error {
	c.mu.Lock()
	r := c.river
	c.river = nil
	c.mu.Unlock()

	if r == nil {
		return nil
	}

	r.Close()
	logctx.With(ctx, c.logger).Info("river stopped", zap.String("event", "river.component.stop.diagnostic"), zap.String("operation", "river.component.stop"))

	return nil
}
