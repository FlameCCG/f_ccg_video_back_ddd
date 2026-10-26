package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestStop_NilSafeAndIdempotent(t *testing.T) {
	// 未启动状态与重复 Stop 均不应 panic。
	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	component.Stop(context.Background())

	component.Stop(context.Background())

	// nil 组件同样安全。
	var nilComponent *CronComponent
	nilComponent.Stop(context.Background())
}

func TestStop_StopsSchedulerAndRunningJob(t *testing.T) {
	var (
		started atomic.Bool
		exited  atomic.Bool
	)

	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	// 注册一个长任务：启动后阻塞，直到 Stop 的 context 等待窗口结束或任务自行返回。
	// 这里用短 sleep，确保 crontab.Stop() 的 Done 能在任务结束后触发。
	component.startJobs([]cronJob{{
		name: "TestJob",
		spec: "* * * * * *",
		run: func() {
			started.Store(true)
			time.Sleep(50 * time.Millisecond)
			exited.Store(true)
		},
	}})

	// 等至少一次调度触发。
	deadline := time.Now().Add(2 * time.Second)
	for !started.Load() {
		if time.Now().After(deadline) {
			t.Fatal("cron job did not start within timeout")
		}

		time.Sleep(10 * time.Millisecond)
	}

	component.Stop(context.Background())

	if component.crontab != nil {
		t.Fatal("crontab should be cleared after Stop")
	}

	// Stop 应等待在途任务结束（本任务约 50ms）。
	if !exited.Load() {
		t.Fatal("in-flight cron job should finish before Stop returns")
	}

	// 幂等。
	component.Stop(context.Background())
}

func TestStartJobs_IgnoresDuplicateStart(t *testing.T) {
	var runs atomic.Int64

	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	job := []cronJob{{
		name: "TestJob",
		spec: "* * * * * *",
		run:  func() { runs.Add(1) },
	}}

	component.startJobs(job)

	first := component.crontab

	component.startJobs(job)

	if component.crontab != first {
		t.Fatal("duplicate start should not replace the running scheduler")
	}

	component.Stop(context.Background())
}

func TestCronSpecsMatchLegacySchedule(t *testing.T) {
	// cron 表达式属外部契约，逐条与原 service/cron_service/enter.go 核对。
	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	want := []struct {
		name string
		spec string
	}{
		{name: "SyncCommentLike", spec: "*/30 * * * * ?"},
		{name: "SyncVideoStat", spec: "0 */5 * * * ?"},
		{name: "DecayVideoHotScore", spec: "0 */10 * * * ?"},
		{name: "AutoUnbanUsers", spec: "0 */5 * * * ?"},
	}

	jobs := component.jobs()
	if len(jobs) != len(want) {
		t.Fatalf("expected %d cron jobs, got %d", len(want), len(jobs))
	}

	for i, expected := range want {
		if jobs[i].name != expected.name || jobs[i].spec != expected.spec {
			t.Fatalf("job %d: expected %s/%q, got %s/%q", i, expected.name, expected.spec, jobs[i].name, jobs[i].spec)
		}
	}
}

type uploadCleanupRecorder struct{ calls atomic.Int32 }

func (r *uploadCleanupRecorder) Run(context.Context) { r.calls.Add(1) }

func TestUploadCleanupRunsEveryFiveMinutes(t *testing.T) {
	recorder := &uploadCleanupRecorder{}
	component := NewCronComponent(nil, nil, nil, nil, recorder, zap.NewNop())
	jobs := component.jobs()
	if len(jobs) != 5 || jobs[4].name != "CleanupUploads" || jobs[4].spec != "0 */5 * * * ?" {
		t.Fatalf("upload cleanup schedule = %+v", jobs)
	}
	jobs[4].run()
	if recorder.calls.Load() != 1 {
		t.Fatal("upload cleanup job did not run")
	}
}
