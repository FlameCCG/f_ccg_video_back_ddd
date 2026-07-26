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
