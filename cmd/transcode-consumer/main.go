// Package main 是独立 Kafka 转码消费者的进程入口。
//
// 它复用 bootstrap.InitializeTranscodeWorker 和 media 用例，不装配 HTTP 路由、River 或站点 cron。
// 运行上下文由退出信号取消，worker 先停止消费与任务，再由 Wire cleanup 回收连接。
// 与主服务同时部署时应结合 Kafka consumer group 与本地消费开关理解任务分配。
package main

import (
	"context"
	"f_ccg_video_back_ddd/internal/bootstrap"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(run())
}

func run() int {
	file := flag.String("f", "config_dev.yaml", "配置文件")

	flag.Parse()

	if flag.NArg() != 0 {
		_, _ = fmt.Fprintln(os.Stderr, "用法: transcode-consumer [-f 配置文件]")
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	worker, cleanup, err := bootstrap.InitializeTranscodeWorker(&cli.Options{File: *file})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "初始化转码消费者失败: %v\n", logsafe.Text(logsafe.Error(err), logsafe.ErrorLimit))
		return 1
	}
	defer cleanup()

	if err := worker.Run(ctx); err != nil {
		worker.ReportError(err)
		return 1
	}

	return 0
}
