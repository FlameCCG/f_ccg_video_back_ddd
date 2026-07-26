package media

import (
	"context"
	"slices"
	"sync"
)

// TaskKey 标识一次属于某个视频或分P 的转码运行。
type TaskKey struct {
	// VideoID 视频 ID。
	VideoID uint
	// PartID 分P ID；0 表示单P视频。
	PartID uint
}

// TaskRegistry 持有当前进程内所有转码运行的取消函数。
//
// 同一个 key 可以对应多次运行，避免重复投递覆盖掉更早的取消函数。
// 原实现为 transcode_cancel_service 的包级 DefaultRegistry；
// 新架构改为可注入实例，由 bootstrap 创建唯一对象。
type TaskRegistry struct {
	mu      sync.Mutex
	nextID  uint64
	running map[TaskKey]map[uint64]context.CancelCauseFunc
}

// NewTaskRegistry 创建空的任务登记表。
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{running: make(map[TaskKey]map[uint64]context.CancelCauseFunc)}
}

// Register 派生可取消的任务上下文，并返回幂等的注销函数。
func (r *TaskRegistry) Register(parent context.Context, key TaskKey) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancelCause(parent)
	if r == nil || key.VideoID == 0 {
		return ctx, func() { cancel(context.Canceled) }
	}

	r.mu.Lock()
	r.nextID++
	runID := r.nextID

	runs := r.running[key]
	if runs == nil {
		runs = make(map[uint64]context.CancelCauseFunc)
		r.running[key] = runs
	}

	runs[runID] = cancel
	r.mu.Unlock()

	var once sync.Once

	unregister := func() {
		once.Do(func() {
			r.mu.Lock()

			if runs := r.running[key]; runs != nil {
				delete(runs, runID)

				if len(runs) == 0 {
					delete(r.running, key)
				}
			}

			r.mu.Unlock()
			cancel(context.Canceled)
		})
	}

	return ctx, unregister
}

// CancelVideos 取消与给定视频 ID 相关的全部本地运行，返回被取消的运行数。
func (r *TaskRegistry) CancelVideos(videoIDs []uint) int {
	if r == nil || len(videoIDs) == 0 {
		return 0
	}

	ids := make(map[uint]struct{}, len(videoIDs))

	for _, videoID := range videoIDs {
		if videoID > 0 {
			ids[videoID] = struct{}{}
		}
	}

	r.mu.Lock()

	cancels := make([]context.CancelCauseFunc, 0)

	for key, runs := range r.running {
		if _, ok := ids[key.VideoID]; !ok {
			continue
		}

		for _, cancel := range runs {
			cancels = append(cancels, cancel)
		}
	}

	r.mu.Unlock()

	for _, cancel := range cancels {
		cancel(ErrVideoDeleted)
	}

	return len(cancels)
}

// ActiveCount 返回本地运行数，供测试与诊断使用。
func (r *TaskRegistry) ActiveCount() int {
	if r == nil {
		return 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, runs := range r.running {
		count += len(runs)
	}

	return count
}

// NormalizeVideoIDs 去重、排序并剔除 0 值，得到稳定的广播载荷。
func NormalizeVideoIDs(videoIDs []uint) []uint {
	ids := append([]uint(nil), videoIDs...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	ids = slices.DeleteFunc(ids, func(id uint) bool { return id == 0 })

	return ids
}
