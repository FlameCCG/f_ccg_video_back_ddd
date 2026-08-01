package cli

import "testing"

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
	if got := ResolveTask(all); got != TaskMigrateDB {
		t.Fatalf("all flags: got %v, want TaskMigrateDB", got)
	}

	cases := []struct {
		name string
		opts *Options
		want Task
	}{
		{"nil options", nil, TaskServe},
		{"empty options", &Options{}, TaskServe},
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

			if got := ResolveTask(tt.opts); got != tt.want {
				t.Fatalf("ResolveTask(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestInfraScopePerTask 校验各任务所需的基础设施范围与原实现的初始化次序一致。
func TestInfraScopePerTask(t *testing.T) {
	t.Parallel()

	dbOnly := []Task{TaskMigrateDB, TaskVideoPartitions}
	for _, task := range dbOnly {
		if !NeedsDBOnly(task) {
			t.Fatalf("task %v should only need config+logger+mysql", task)
		}

		if NeedsStorage(task) || NeedsCasbin(task) || NeedsES(task) || NeedsFullInfra(task) {
			t.Fatalf("task %v should not need storage, Casbin, ES or full infra", task)
		}
	}

	if !NeedsStorage(TaskBanner) || NeedsDBOnly(TaskBanner) || NeedsCasbin(TaskBanner) ||
		NeedsES(TaskBanner) || NeedsFullInfra(TaskBanner) {
		t.Fatal("TaskBanner should need config+logger+mysql+storage only")
	}

	if !NeedsCasbin(TaskSuperUser) || NeedsDBOnly(TaskSuperUser) || NeedsStorage(TaskSuperUser) ||
		NeedsES(TaskSuperUser) || NeedsFullInfra(TaskSuperUser) {
		t.Fatal("TaskSuperUser should need config+logger+mysql+Casbin only")
	}

	if !NeedsES(TaskESIndex) || NeedsDBOnly(TaskESIndex) || NeedsFullInfra(TaskESIndex) {
		t.Fatal("TaskESIndex should need config+logger+mysql+es only")
	}

	for _, task := range []Task{TaskResourceSync, TaskRBACInit} {
		if !NeedsFullInfra(task) {
			t.Fatalf("task %v should need full infrastructure", task)
		}

		if NeedsDBOnly(task) || NeedsES(task) {
			t.Fatalf("task %v scope classification is wrong", task)
		}
	}

	if NeedsDBOnly(TaskServe) || NeedsStorage(TaskServe) || NeedsCasbin(TaskServe) ||
		NeedsES(TaskServe) || NeedsFullInfra(TaskServe) {
		t.Fatal("TaskServe is not a CLI maintenance task")
	}
}
