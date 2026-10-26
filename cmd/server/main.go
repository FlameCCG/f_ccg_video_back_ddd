// Package main 是 HTTP 服务与一次性运维任务的进程入口。
//
// 先读 run 的任务分流，再读 runServer 的装配、启动与停机；业务规则不放在 main 中。
// InitializeApp 返回的 cleanup 负责依赖清理，App.Start/Serve/Stop 负责运行期启停。
// main 最后才调用 os.Exit：若在用例中直接退出，调用栈中的 defer 和 Wire cleanup 都不会执行。
// -all 按步骤分别装配任务，每步清理后才进入下一步；它不是跨步骤数据库事务。
package main

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/bootstrap"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"os"
	"strings"
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
	if task == cli.TaskInitAll {
		allOpts := *opts
		if len(allOpts.Args) < 2 || strings.TrimSpace(allOpts.Args[0]) == "" || strings.TrimSpace(allOpts.Args[1]) == "" {
			_, _ = fmt.Fprintln(os.Stderr, "请同时提供超级管理员用户名和密码，例如: go run ./cmd/server -all 用户名 密码")
			return 1
		}
		// 每步独立装配并清理，确保迁移完成后才构造依赖数据库表的基础设施。
		for _, step := range []struct {
			name string
			task cli.Task
		}{
			{"db", cli.TaskMigrateDB},
			{"vp", cli.TaskVideoPartitions},
			{"br", cli.TaskBanner},
			{"es", cli.TaskESIndex},
			{"rs", cli.TaskResourceSync},
			{"rbac", cli.TaskRBACInit},
			{"su", cli.TaskSuperUser},
		} {
			_, _ = fmt.Fprintf(os.Stderr, "初始化资源: %s\n", step.name)
			if code := runMaintenanceTaskWithInitializer(ctx, step.task, &allOpts, initialize); code != 0 {
				_, _ = fmt.Fprintf(os.Stderr, "初始化资源失败: %s\n", step.name)
				return code
			}
		}

		return 0
	}

	executor, cleanup, err := initialize(task, opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化运维任务失败: %v\n", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))
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
		return nil, nil, errors.New("serve is not a maintenance task")
	case cli.TaskInitAll:
		fallthrough
	default:
		return nil, nil, fmt.Errorf("unknown maintenance task: %d", task)
	}
}

func runServer(ctx context.Context, opts *cli.Options) int {
	app, cleanup, err := bootstrap.InitializeApp(opts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化应用失败: %v\n", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))
		return 1
	}

	if cleanup != nil {
		defer cleanup()
	}

	exitCode := 0

	if err := app.Start(ctx); err != nil {
		app.ReportError(err)

		exitCode = 1
	}

	// Serve 阻塞至 SIGINT/SIGTERM（或 ListenAndServe 异常退出），期间完成 HTTP 优雅关闭。
	// 随后显式按序停止后台消费者与基础设施，避免仅依赖 defer 在硬杀时不可达。
	if err := app.Serve(); err != nil {
		app.ReportError(err)

		exitCode = 1
	}

	if err := app.Stop(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "优雅退出失败: %v\n", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))
		exitCode = 1
	}

	return exitCode
}
