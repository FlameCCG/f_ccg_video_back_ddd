//go:build wireinject
// +build wireinject

package bootstrap

import (
	"f_ccg_video_back_ddd/internal/interfaces/cli"

	"github.com/google/wire"
)

// InitializeApp 构造完整应用（HTTP 服务 + 后台组件 + 基础设施）。
//
// 生成命令（见 docs/refactor-spec.md 第 2 节）：
//
//	go run github.com/google/wire/cmd/wire ./internal/bootstrap
func InitializeApp(opts *cli.Options) (*App, func(), error) {
	panic(wire.Build(AppProviderSet))
}

// InitializeDBTasks 构造只需要「配置 + 日志 + MySQL + Casbin」的运维任务集合，
// 服务 -db / -vp / -su（-br 需要对象存储，随 media 上下文接入）。
//
// 依赖范围严格等于原 core.InitCore 中 flags.RunDBTasks 执行点之前已初始化的组件。
func InitializeDBTasks(opts *cli.Options) (*DBTasks, func(), error) {
	panic(wire.Build(DBTasksProviderSet))
}
