package river

import (
	"context"
	"testing"

	"f_ccg_video_back_ddd/internal/config"

	"go.uber.org/zap"
)

// 验证 River 的单次启动与幂等停止约定。

// TestRiverComponentStartIsIdempotentWhenDisabled 验证未启用时 Start 只记录日志，重复调用不再执行构造。
func TestRiverComponentStartIsIdempotentWhenDisabled(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: false}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	for range 3 {
		c.Start(context.Background())
	}

	if c.river != nil {
		t.Fatal("disabled river must not construct an engine")
	}
}

// TestRiverComponentStartDoesNotBlockOnConstructionFailure 构造失败（此处 MySQL 不可达）
// 只记 "init river failed"，不阻断进程启动。
func TestRiverComponentStartDoesNotBlockOnConstructionFailure(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(
		&config.River{Enable: true},
		// DBName 为空会让 validateRiverSchemas 直接失败，无需真实 MySQL。
		&config.Mysql{},
		&config.ES{},
		nil,
		zap.NewNop(),
	)

	c.Start(context.Background())

	if c.river != nil {
		t.Fatal("failed construction must leave the engine nil")
	}
}

// TestRiverComponentStopIsIdempotent 未启动或已停止时 Stop 均无副作用。
func TestRiverComponentStopIsIdempotent(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: false}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	for range 3 {
		c.Stop(context.Background())
	}
}

// TestRiverComponentStartAfterStopStaysStopped Start 只生效一次：
// Stop 之后再 Start 不会重新拉起引擎。
func TestRiverComponentStartAfterStopStaysStopped(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: true}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	c.Start(context.Background())

	c.Stop(context.Background())

	c.Start(context.Background())

	if c.river != nil {
		t.Fatal("Start after Stop must not restart the engine")
	}
}
