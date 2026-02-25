package cli

// Task 表示本次进程要执行的动作。
type Task int

// 任务取值。
const (
	// TaskServe 启动 HTTP 服务与后台组件（默认）。
	TaskServe Task = iota
	// TaskMigrateDB 执行数据库迁移（-db）。
	TaskMigrateDB
	// TaskVideoPartitions 初始化视频分区数据（-vp）。
	TaskVideoPartitions
	// TaskBanner 初始化 Banner 数据（-br）。
	TaskBanner
	// TaskSuperUser 创建超级管理员账户（-su）。
	TaskSuperUser
	// TaskESIndex 创建搜索索引（-es）。
	TaskESIndex
	// TaskResourceSync 同步接口资源（-rs）。
	TaskResourceSync
	// TaskRBACInit 初始化 RBAC 默认数据（-rbac）。
	TaskRBACInit
	// TaskInitAll 按顺序初始化全部资源及超级管理员（-all）。
	TaskInitAll
)

// ResolveTask 依据命令行选项决定执行任务。
//
// 优先级为 -db → -vp → -br → -su → -es → -rs → -rbac；
// 多个开关同时给出时选择靠前项，-all 优先于所有单项。
func ResolveTask(opts *Options) Task {
	if opts == nil {
		return TaskServe
	}

	switch {
	case opts.All:
		return TaskInitAll
	case opts.DB:
		return TaskMigrateDB
	case opts.VideoPartitions:
		return TaskVideoPartitions
	case opts.Banner:
		return TaskBanner
	case opts.SuperUser:
		return TaskSuperUser
	case opts.Index:
		return TaskESIndex
	case opts.RS:
		return TaskResourceSync
	case opts.RBAC:
		return TaskRBACInit
	default:
		return TaskServe
	}
}

// NeedsDBOnly 报告任务是否仅需配置、日志与 MySQL；-db 和 -vp 不访问其它基础设施。
func NeedsDBOnly(task Task) bool {
	switch task {
	case TaskMigrateDB, TaskVideoPartitions:
		return true
	default:
		return false
	}
}

// NeedsStorage 报告任务是否还需站点存储驱动；-br 不初始化 ES、Redis、Mongo、IPDB 或 Casbin。
func NeedsStorage(task Task) bool {
	return task == TaskBanner
}

// NeedsCasbin 报告任务是否还需 Casbin；-su 按需创建 enforcer，其它数据库任务不承担此依赖。
func NeedsCasbin(task Task) bool {
	return task == TaskSuperUser
}

// NeedsES 报告任务是否需要配置、日志、MySQL 与 ES；-es 在 ES 初始化后、Redis 初始化前执行。
func NeedsES(task Task) bool {
	return task == TaskESIndex
}

// NeedsFullInfra 报告任务是否需要全部基础设施，包括 Redis、Mongo、IPDB 和 Casbin。
func NeedsFullInfra(task Task) bool {
	switch task {
	case TaskResourceSync, TaskRBACInit:
		return true
	default:
		return false
	}
}
