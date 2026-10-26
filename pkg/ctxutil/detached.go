package ctxutil

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logctx"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DetachedTaskGroup 管理与请求解耦、但仍受进程生命周期约束的后台任务。
//
// 原实现为 pkg/ctxutil 的包级单例（processDetachedTasks）+ 三个包级函数；
// 新架构改为可注入类型：由 bootstrap 创建唯一实例并注入需要 fire-and-forget 的应用服务，
// 优雅退出时由 App.Stop 调用 Stop 取消并等待在途任务。行为与原实现一致。
type DetachedTaskGroup struct {
	logger     *zap.Logger
	mu         sync.Mutex
	active     int
	idle       chan struct{}
	stopping   bool
	rootCtx    context.Context //nolint:containedctx // Owns the task group lifecycle; Stop cancels all pending detached work.
	rootCancel context.CancelFunc
}

// NewDetachedTaskGroup 创建一个 detached 任务组。
func NewDetachedTaskGroup() *DetachedTaskGroup {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	idle := make(chan struct{})
	close(idle)

	return &DetachedTaskGroup{
		logger:     zap.NewNop(),
		idle:       idle,
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
	}
}

// Go 在后台运行一个有截止时间、且受进程生命周期管理的任务。
// 进程开始停止后，新任务会被拒绝；返回值表示任务是否被接受。
func (g *DetachedTaskGroup) Go(parent context.Context, timeout time.Duration, fn func(context.Context)) bool {
	if g == nil || fn == nil {
		return false
	}

	g.mu.Lock()

	if g.stopping {
		g.mu.Unlock()

		return false
	}

	if g.active == 0 {
		g.idle = make(chan struct{})
	}

	values := logmeta.From(parent)
	if values.TaskID == "" {
		values.TaskID = logmeta.NewID()
	}

	values.AttemptID = logmeta.NewID()
	ctx, cancel := WithDetachedTimeout(logmeta.With(parent, values), timeout)
	stopCancel := context.AfterFunc(g.rootCtx, cancel)
	g.active++
	g.mu.Unlock()

	go func() {
		defer g.done()
		defer stopCancel()
		defer cancel()

		logger := logctx.With(ctx, g.logger)
		start := time.Now()

		logger.Info("task.started", zap.String("event", "task.started"))

		defer func() {
			fields := []zap.Field{zap.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond))}
			if value := recover(); value != nil {
				logger.Error("task.failed", append(fields, zap.String("event", "task.failed"), zap.String("error", logsafe.Text(fmt.Sprint(value), logsafe.ErrorLimit)), zap.String("stack", string(debug.Stack())))...)
				return
			}

			event := "task.finished"
			if ctx.Err() != nil {
				event = "task.canceled"
			}

			logger.Info(event, append(fields, zap.String("event", event))...)
		}()

		fn(ctx)
	}()

	return true
}

// Wait 等待当前及等待期间注册的 detached 任务全部结束。
// 该方法不会阻止后续任务注册，主要用于测试清理和显式同步点。
func (g *DetachedTaskGroup) Wait(ctx context.Context) error {
	if g == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	for {
		g.mu.Lock()

		if g.active == 0 {
			g.mu.Unlock()

			return nil
		}

		idle := g.idle
		g.mu.Unlock()

		select {
		case <-idle:
			// 任务结束回调可能继续登记任务，重新检查是否真正空闲。
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Stop 拒绝新任务、取消所有在途 detached 任务并等待其退出。
// 该操作是幂等的，且任务组生命周期内不可恢复。
func (g *DetachedTaskGroup) Stop(ctx context.Context) error {
	if g == nil {
		return nil
	}

	g.mu.Lock()

	if !g.stopping {
		g.stopping = true
		g.rootCancel()
	}

	g.mu.Unlock()

	return g.Wait(ctx)
}

func (g *DetachedTaskGroup) done() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.active--
	if g.active == 0 {
		close(g.idle)
	}
}

// NewLoggedDetachedTaskGroup injects the process logger without storing it in a context.
func NewLoggedDetachedTaskGroup(logger *zap.Logger) *DetachedTaskGroup {
	g := NewDetachedTaskGroup()
	if logger != nil {
		g.logger = logger
	}

	return g
}
