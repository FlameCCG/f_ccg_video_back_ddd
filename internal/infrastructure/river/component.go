package river

import (
	"context"
	"sync"

	"f_ccg_video_back_ddd/internal/config"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RiverComponent 管理 MySQL binlog 到 ES 同步引擎的后台生命周期。
//
// Start 仅生效一次；River 未启用时记录日志并跳过；构造失败只记错误，不阻断进程启动。
// 启动后在后台执行 r.Run()。Stop 幂等，启停均记录对应状态日志。
type RiverComponent struct {
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
) *RiverComponent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &RiverComponent{
		riverCfg: riverCfg,
		mysqlCfg: mysqlCfg,
		esCfg:    esCfg,
		db:       db,
		logger:   logger,
	}
}

// Start 启动 binlog 同步；重复调用无副作用，启动失败不返回错误。
func (c *RiverComponent) Start(_ context.Context) {
	c.mu.Lock()

	if c.started {
		c.mu.Unlock()

		return
	}

	c.started = true
	c.mu.Unlock()

	if c.riverCfg == nil || !c.riverCfg.Enable {
		c.logger.Info("river disabled")

		return
	}

	r, err := NewRiver(c.riverCfg, c.mysqlCfg, c.esCfg, c.db)
	if err != nil {
		c.logger.Error("init river failed", zap.Error(err))

		return
	}

	c.mu.Lock()
	c.river = r
	c.mu.Unlock()

	go func() {
		if err := r.Run(); err != nil {
			c.logger.Error("river stopped", zap.Error(err))
		}
	}()

	c.logger.Info("river started")
}

// Stop 优雅关闭同步引擎：取消上下文、关闭 canal、强制刷盘 binlog 位点（幂等）。
func (c *RiverComponent) Stop(_ context.Context) {
	c.mu.Lock()
	r := c.river
	c.river = nil
	c.mu.Unlock()

	if r == nil {
		return
	}

	r.Close()
	c.logger.Info("river stopped")
}
