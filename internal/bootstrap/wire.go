//go:build wireinject
// +build wireinject

package bootstrap

import (
	"f_ccg_video_back_ddd/internal/interfaces/cli"

	"github.com/google/wire"
)

// InitializeApp 构造完整应用（HTTP 服务 + 后台组件 + 基础设施）。
//
// 本文件带 wireinject 标签，只供 Wire 分析；这里的 panic(wire.Build(...)) 是生成声明，
// 不会进入正常服务的运行路径。普通构建使用 wire_gen.go 中同名的普通 Go 函数。
// 返回的 func() 是清理已成功构造依赖的闭包，error 对应返回错误的 provider；
// provider 自身发生 panic（如配置加载失败）时，不应假定会走普通 error 回退路径。
//
// 生成命令（在项目根目录执行；依赖图说明见 docs/architecture.md 第 6 节）：
//
//	go run -mod=mod github.com/google/wire/cmd/wire ./internal/bootstrap
func InitializeApp(opts *cli.Options) (*App, func(), error) {
	panic(wire.Build(AppProviderSet))
}

// InitializeTranscodeWorker 构造独立转码消费者，不装配 HTTP 与其他后台组件。
func InitializeTranscodeWorker(opts *cli.Options) (*TranscodeWorker, func(), error) {
	panic(wire.Build(TranscodeWorkerProviderSet))
}

// InitializeMigrateDBTask 构造 -db 运维任务（配置 + 日志 + MySQL）。
func InitializeMigrateDBTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(MigrateDBTaskProviderSet))
}

// InitializeVideoPartitionsTask 构造 -vp 运维任务（配置 + 日志 + MySQL）。
func InitializeVideoPartitionsTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(VideoPartitionsTaskProviderSet))
}

// InitializeBannerTask 构造 -br 运维任务（配置 + 日志 + MySQL + 存储）。
func InitializeBannerTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(BannerTaskProviderSet))
}

// InitializeSuperUserTask 构造 -su 运维任务（配置 + 日志 + MySQL + Casbin）。
func InitializeSuperUserTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(SuperUserTaskProviderSet))
}

// InitializeESIndexTask 构造 -es 运维任务（配置 + 日志 + MySQL + Elasticsearch）。
func InitializeESIndexTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(ESIndexTaskProviderSet))
}

// InitializeResourceSyncTask 构造 -rs 运维任务（完整前置基础设施 + 真实 HTTP Router）。
func InitializeResourceSyncTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(ResourceSyncTaskProviderSet))
}

// InitializeRBACInitTask 构造 -rbac 运维任务（完整前置基础设施）。
func InitializeRBACInitTask(opts *cli.Options) (*cli.Executor, func(), error) {
	panic(wire.Build(RBACInitTaskProviderSet))
}
