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

// ResolveTask 依据命令行选项决定要执行的任务。
//
// 优先级严格复刻原 core.InitCore 的执行顺序：
// flags.RunDBTasks 依次检查 -db → -vp → -br → -su（各自 os.Exit），
// 随后 flags.RunESTasks 检查 -es，最后 InitCore 依次检查 -rs → -rbac。
// 因此多个开关同时给出时，靠前者胜出；-all 优先于所有单项开关。
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

// NeedsDBOnly 报告该任务是否只需要「配置 + 日志 + MySQL」这一层基础设施。
//
// -db/-vp 在原 flags.RunDBTasks 中不访问其它基础设施。
func NeedsDBOnly(task Task) bool {
	switch task {
	case TaskMigrateDB, TaskVideoPartitions:
		return true
	case TaskServe, TaskBanner, TaskSuperUser, TaskESIndex, TaskResourceSync, TaskRBACInit, TaskInitAll:
		fallthrough
	default:
		return false
	}
}

// NeedsStorage 报告任务是否在数据库基础上还需要站点存储驱动。
// 原 -br 会在任务内部 ResolveDriver，但不会初始化 ES/Redis/Mongo/IPDB/Casbin。
func NeedsStorage(task Task) bool {
	return task == TaskBanner
}

// NeedsCasbin 报告任务是否在数据库基础上还需要 Casbin。
// 原 -su 仅在自身执行路径按需创建 enforcer，不应让其它早期数据库任务承担该依赖。
func NeedsCasbin(task Task) bool {
	return task == TaskSuperUser
}

// NeedsES 报告该任务是否需要「配置 + 日志 + MySQL + ES」。
//
// 与原实现一致：-es 在 InitES 之后、InitRedis 之前执行。
func NeedsES(task Task) bool {
	return task == TaskESIndex
}

// NeedsFullInfra 报告该任务是否需要全部基础设施（含 Redis/Mongo/IPDB/Casbin）。
//
// 与原实现一致：-rs/-rbac 在 InitCasbin 之后执行。
func NeedsFullInfra(task Task) bool {
	switch task {
	case TaskResourceSync, TaskRBACInit:
		return true
	case TaskServe, TaskMigrateDB, TaskVideoPartitions, TaskBanner, TaskSuperUser, TaskESIndex, TaskInitAll:
		fallthrough
	default:
		return false
	}
}
