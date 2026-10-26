package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResolveTaskPriority 校验多开关同时给出时的优先级与原 core.InitCore 执行顺序一致：
// -db → -vp → -br → -su → -es → -rs → -rbac。
func TestResolveTaskPriority(t *testing.T) {
	t.Parallel()

	all := &Options{
		DB:              true,
		VideoPartitions: true,
		Banner:          true,
		SuperUser:       true,
		Index:           true,
		RS:              true,
		RBAC:            true,
	}
	{
		got := ResolveTask(all)
		require.Equal(t, TaskMigrateDB, got, "all flags: got %v, want TaskMigrateDB", got)
	}

	cases := []struct {
		name string
		opts *Options
		want Task
	}{
		{"nil options", nil, TaskServe},
		{"empty options", &Options{}, TaskServe},
		{"all resources", &Options{All: true}, TaskInitAll},
		{"all takes priority", &Options{All: true, DB: true, SuperUser: true, Index: true, RBAC: true}, TaskInitAll},
		{"db", &Options{DB: true}, TaskMigrateDB},
		{"video partitions", &Options{VideoPartitions: true, Banner: true, SuperUser: true, Index: true}, TaskVideoPartitions},
		{"banner", &Options{Banner: true, SuperUser: true, Index: true}, TaskBanner},
		{"super user", &Options{SuperUser: true, Index: true, RS: true}, TaskSuperUser},
		{"es index", &Options{Index: true, RS: true, RBAC: true}, TaskESIndex},
		{"resource sync", &Options{RS: true, RBAC: true}, TaskResourceSync},
		{"rbac init", &Options{RBAC: true}, TaskRBACInit},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := ResolveTask(tt.opts)
				require.Equal(t, tt.want, got, "ResolveTask(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestInfraScopePerTask 校验各任务所需的基础设施范围与原实现的初始化次序一致。
func TestInfraScopePerTask(t *testing.T) {
	t.Parallel()

	dbOnly := []Task{TaskMigrateDB, TaskVideoPartitions}
	for _, task := range dbOnly {
		require.True(t, NeedsDBOnly(task), "task %v should only need config+logger+mysql", task)

		require.False(t, NeedsStorage(task), "task %v should not need storage, Casbin, ES or full infra", task)
		require.False(t, NeedsCasbin(task), "task %v should not need storage, Casbin, ES or full infra", task)
		require.False(t, NeedsES(task), "task %v should not need storage, Casbin, ES or full infra", task)
		require.False(t, NeedsFullInfra(task), "task %v should not need storage, Casbin, ES or full infra", task)
	}

	require.True(t, NeedsStorage(TaskBanner), "TaskBanner should need config+logger+mysql+storage only")
	require.False(t, NeedsDBOnly(TaskBanner), "TaskBanner should need config+logger+mysql+storage only")
	require.False(t, NeedsCasbin(TaskBanner), "TaskBanner should need config+logger+mysql+storage only")
	require.False(t, NeedsES(TaskBanner), "TaskBanner should need config+logger+mysql+storage only")
	require.False(t, NeedsFullInfra(TaskBanner), "TaskBanner should need config+logger+mysql+storage only")

	require.True(t, NeedsCasbin(TaskSuperUser), "TaskSuperUser should need config+logger+mysql+Casbin only")
	require.False(t, NeedsDBOnly(TaskSuperUser), "TaskSuperUser should need config+logger+mysql+Casbin only")
	require.False(t, NeedsStorage(TaskSuperUser), "TaskSuperUser should need config+logger+mysql+Casbin only")
	require.False(t, NeedsES(TaskSuperUser), "TaskSuperUser should need config+logger+mysql+Casbin only")
	require.False(t, NeedsFullInfra(TaskSuperUser), "TaskSuperUser should need config+logger+mysql+Casbin only")

	require.True(t, NeedsES(TaskESIndex), "TaskESIndex should need config+logger+mysql+es only")
	require.False(t, NeedsDBOnly(TaskESIndex), "TaskESIndex should need config+logger+mysql+es only")
	require.False(t, NeedsFullInfra(TaskESIndex), "TaskESIndex should need config+logger+mysql+es only")

	for _, task := range []Task{TaskResourceSync, TaskRBACInit} {
		require.True(t, NeedsFullInfra(task), "task %v should need full infrastructure", task)

		require.False(t, NeedsDBOnly(task), "task %v scope classification is wrong", task)
		require.False(t, NeedsES(task), "task %v scope classification is wrong", task)
	}

	require.False(t, NeedsDBOnly(TaskServe), "TaskServe is not a CLI maintenance task")
	require.False(t, NeedsStorage(TaskServe), "TaskServe is not a CLI maintenance task")
	require.False(t, NeedsCasbin(TaskServe), "TaskServe is not a CLI maintenance task")
	require.False(t, NeedsES(TaskServe), "TaskServe is not a CLI maintenance task")
	require.False(t, NeedsFullInfra(TaskServe), "TaskServe is not a CLI maintenance task")
}
