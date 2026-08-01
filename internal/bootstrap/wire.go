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
