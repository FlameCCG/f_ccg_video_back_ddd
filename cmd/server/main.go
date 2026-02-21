// Package main 是 f_ccg_video_back_ddd 的应用入口。
//
// 职责仅限：解析 CLI flags → 调 bootstrap 注入器 → 运行并等待退出信号。
package main

import (
	"context"
	"f_ccg_video_back_ddd/internal/bootstrap"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"fmt"
	"os"
)

func main() {
	os.Exit(run(cli.ParseOptions()))
}

func run(opts *cli.Options) int {
	task := cli.ResolveTask(opts)
	if task != cli.TaskServe {
		return runMaintenanceTask(context.Background(), task, opts)
	}

	return runServer(context.Background(), opts)
}

func runMaintenanceTask(ctx context.Context, task cli.Task, opts *cli.Options) int {
	return runMaintenanceTaskWithInitializer(ctx, task, opts, initializeMaintenanceTask)
}

type maintenanceTaskInitializer func(
	cli.Task,
	*cli.Options,
) (*cli.Executor, func(), error)

func runMaintenanceTaskWithInitializer(
	ctx context.Context,
	task cli.Task,
	opts *cli.Options,
	initialize maintenanceTaskInitializer,
) int {
	executor, cleanup, err := initialize(task, opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化运维任务失败: %v\n", err)

		return 1
	}

	if cleanup != nil {
		defer cleanup()
	}

	if err := executor.Execute(ctx, opts.Args); err != nil {
		return 1
	}

	return 0
}

func initializeMaintenanceTask(
	task cli.Task,
	opts *cli.Options,
) (*cli.Executor, func(), error) {
	switch task {
	case cli.TaskMigrateDB:
		return bootstrap.InitializeMigrateDBTask(opts)
	case cli.TaskVideoPartitions:
		return bootstrap.InitializeVideoPartitionsTask(opts)
	case cli.TaskBanner:
		return bootstrap.InitializeBannerTask(opts)
	case cli.TaskSuperUser:
		return bootstrap.InitializeSuperUserTask(opts)
	case cli.TaskESIndex:
		return bootstrap.InitializeESIndexTask(opts)
	case cli.TaskResourceSync:
		return bootstrap.InitializeResourceSyncTask(opts)
	case cli.TaskRBACInit:
		return bootstrap.InitializeRBACInitTask(opts)
	case cli.TaskServe:
		return nil, nil, fmt.Errorf("serve is not a maintenance task")
	default:
		return nil, nil, fmt.Errorf("unknown maintenance task: %d", task)
	}
}

func runServer(ctx context.Context, opts *cli.Options) int {

	app, cleanup, err := bootstrap.InitializeApp(opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化应用失败: %v\n", err)

		return 1
	}

	if cleanup != nil {
		defer cleanup()
	}

	exitCode := 0

	if err := app.Start(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "启动后台组件失败: %v\n", err)
		exitCode = 1
	}

	// Serve 阻塞至 SIGINT/SIGTERM（或 ListenAndServe 异常退出），期间完成 HTTP 优雅关闭。
	// 随后显式按序停止后台消费者与基础设施，避免仅依赖 defer 在硬杀时不可达。
	if err := app.Serve(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "HTTP 服务退出异常: %v\n", err)
		exitCode = 1
	}

	if err := app.Stop(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "优雅退出失败: %v\n", err)
		exitCode = 1
	}

	return exitCode
}
