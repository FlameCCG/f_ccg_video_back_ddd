// Command transcode-consumer 只运行 Kafka 转码消费者。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"f_ccg_video_back_ddd/internal/bootstrap"
	"f_ccg_video_back_ddd/internal/interfaces/cli"
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
		_, _ = fmt.Fprintf(os.Stderr, "初始化转码消费者失败: %v\n", err)
		return 1
	}
	defer cleanup()

	if err := worker.Run(ctx); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "转码消费者退出异常: %v\n", err)
		return 1
	}
	return 0
}
