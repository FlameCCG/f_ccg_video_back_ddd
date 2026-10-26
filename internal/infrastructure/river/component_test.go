package river

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"testing"

	"go.uber.org/zap"
)

// 对应原 core/init_river.go 的 sync.Once + 幂等 Stop 语义。

// TestRiverComponentStartIsIdempotentWhenDisabled 未启用时 Start 只记日志、返回 nil，
// 且重复调用不再进入构造分支（等价原 sync.Once）。
func TestRiverComponentStartIsIdempotentWhenDisabled(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: false}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	for range 3 {
		if err := c.Start(context.Background()); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}

	if c.river != nil {
		t.Fatal("disabled river must not construct an engine")
	}
}

// TestRiverComponentStartDoesNotBlockOnConstructionFailure 构造失败（此处 MySQL 不可达）
// 只记 "init river failed"，Start 仍返回 nil，不阻断进程启动。
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

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start must not return an error on construction failure: %v", err)
	}

	if c.river != nil {
		t.Fatal("failed construction must leave the engine nil")
	}
}

// TestRiverComponentStopIsIdempotent 未启动或已停止时 Stop 均返回 nil。
func TestRiverComponentStopIsIdempotent(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: false}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	for range 3 {
		if err := c.Stop(context.Background()); err != nil {
			t.Fatalf("Stop: %v", err)
		}
	}
}

// TestRiverComponentStartAfterStopStaysStopped Start 只生效一次：
// Stop 之后再 Start 不会重新拉起引擎（与原 sync.Once 行为一致）。
func TestRiverComponentStartAfterStopStaysStopped(t *testing.T) {
	t.Parallel()

	c := NewRiverComponent(&config.River{Enable: true}, &config.Mysql{}, &config.ES{}, nil, zap.NewNop())

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("second Start: %v", err)
	}

	if c.river != nil {
		t.Fatal("Start after Stop must not restart the engine")
	}
}
