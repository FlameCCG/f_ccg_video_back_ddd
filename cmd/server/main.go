// Package main 是 f_ccg_video_back_ddd 的应用入口。
//
// 职责仅限：解析 CLI flags → 调 bootstrap 注入器 → 运行并等待退出信号。
package main

import (
	"context"
	"os"

	"f_ccg_video_back_ddd/internal/bootstrap"
	"f_ccg_video_back_ddd/internal/interfaces/cli"

	"go.uber.org/zap"
)

func main() {
	opts := cli.ParseOptions()

	app, cleanup, err := bootstrap.InitializeApp(opts)
	if err != nil {
		zap.L().Error("初始化应用失败", zap.Error(err))
		os.Exit(1)
	}

	defer cleanup()

	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		zap.L().Error("启动后台组件失败", zap.Error(err))
	}

	// Serve 阻塞至 SIGINT/SIGTERM（或 ListenAndServe 异常退出），期间完成 HTTP 优雅关闭。
	// 随后显式按序停止后台消费者与基础设施，避免仅依赖 defer 在硬杀时不可达。
	app.Serve()

	if err := app.Stop(ctx); err != nil {
		zap.L().Error("优雅退出失败", zap.Error(err))
	}
}
