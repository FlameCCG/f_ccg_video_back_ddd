package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStop_NilSafeAndIdempotent(t *testing.T) {
	t.Parallel()

	// 未启动状态与重复 Stop 均不应 panic。
	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	{
		err := component.Stop(context.Background())
		require.NoError(t, err, "Stop: %v", err)
	}

	{
		err := component.Stop(context.Background())
		require.NoError(t, err, "Stop: %v", err)
	}

	// nil 组件同样安全。
	var nilComponent *CronComponent
	{
		err := nilComponent.Stop(context.Background())
		require.NoError(t, err, "Stop on nil component: %v", err)
	}
}

func TestStop_StopsSchedulerAndRunningJob(t *testing.T) {
	t.Parallel()

	var (
		started atomic.Bool
		exited  atomic.Bool
	)

	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	// 注册一个长任务：启动后阻塞，直到 Stop 的 context 等待窗口结束或任务自行返回。
	// 这里用短 sleep，确保 crontab.Stop() 的 Done 能在任务结束后触发。
	{
		err := component.startJobs([]cronJob{{
			name: "TestJob",
			spec: "* * * * * *",
			run: func(context.Context) {
				started.Store(true)
				time.Sleep(50 * time.Millisecond)
				exited.Store(true)
			},
		}})
		require.NoError(t, err, "startJobs: %v", err)
	}

	// 等至少一次调度触发。
	deadline := time.Now().Add(2 * time.Second)
	for !started.Load() {
		require.False(t, time.Now().After(deadline), "cron job did not start within timeout")

		time.Sleep(10 * time.Millisecond)
	}

	{
		err := component.Stop(context.Background())
		require.NoError(t, err, "Stop: %v", err)
	}

	require.Nil(t, component.crontab, "crontab should be cleared after Stop")

	// Stop 应等待在途任务结束（本任务约 50ms）。
	require.True(t, exited.Load(), "in-flight cron job should finish before Stop returns")

	// 幂等。
	{
		err := component.Stop(context.Background())
		require.NoError(t, err, "Stop: %v", err)
	}
}

func TestStartJobs_IgnoresDuplicateStart(t *testing.T) {
	t.Parallel()

	var runs atomic.Int64

	component := NewCronComponent(nil, nil, nil, nil, nil, zap.NewNop())

	job := []cronJob{{
		name: "TestJob",
		spec: "* * * * * *",
		run:  func(context.Context) { runs.Add(1) },
	}}

	{
		err := component.startJobs(job)
		require.NoError(t, err, "startJobs: %v", err)
	}

	first := component.crontab

	{
		err := component.startJobs(job)
		require.NoError(t, err, "startJobs (duplicate): %v", err)
	}

	require.Same(t, component.crontab, first, "duplicate start should not replace the running scheduler")

	{
		err := component.Stop(context.Background())
		require.NoError(t, err, "Stop: %v", err)
	}
}

func TestCronSpecsMatchLegacySchedule(t *testing.T) {
	t.Parallel()

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
	require.Len(t, jobs, len(want), "expected %d cron jobs, got %d", len(want), len(jobs))

	for i, expected := range want {
		require.Equal(t, expected.name, jobs[i].name, "job %d: expected %s/%q, got %s/%q", i, expected.name, expected.spec, jobs[i].name, jobs[i].spec)
		require.Equal(t, expected.spec, jobs[i].spec, "job %d: expected %s/%q, got %s/%q", i, expected.name, expected.spec, jobs[i].name, jobs[i].spec)
	}
}

type uploadCleanupRecorder struct{ calls atomic.Int32 }

func (r *uploadCleanupRecorder) Run(context.Context) { r.calls.Add(1) }

func TestUploadCleanupRunsEveryFiveMinutes(t *testing.T) {
	t.Parallel()

	recorder := &uploadCleanupRecorder{}
	component := NewCronComponent(nil, nil, nil, nil, recorder, zap.NewNop())

	jobs := component.jobs()
	require.Len(t, jobs, 5, "upload cleanup schedule = %+v", jobs)
	require.Equal(t, "CleanupUploads", jobs[4].name, "upload cleanup schedule = %+v", jobs)
	require.Equal(t, "0 */5 * * * ?", jobs[4].spec, "upload cleanup schedule = %+v", jobs)

	jobs[4].run(context.Background())

	require.EqualValues(t, 1, recorder.calls.Load(), "upload cleanup job did not run")
}
