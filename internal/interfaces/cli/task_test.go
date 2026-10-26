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

			if got := ResolveTask(tt.opts); got != tt.want {
				t.Fatalf("ResolveTask(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
