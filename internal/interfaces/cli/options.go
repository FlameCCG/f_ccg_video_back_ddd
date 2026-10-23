// Package cli 解析进程选项并把一次性运维意图交给执行器。
//
// options.go 解析参数，task.go 决定任务优先级，executor.go 运行具体步骤并返回错误。
// 无任务参数时启动服务；各任务在 bootstrap/providers_cli.go 有独立注入器，避免为单项任务建立全部连接。
// executor 不调用 os.Exit，入口收到结果后才决定退出码，使 Wire cleanup 和日志刷盘仍能执行。
// -all 的顺序在 cmd/server 组织；涉及索引重建等操作时应先阅读对应任务实现。
package cli

import "flag"

// Options 命令行选项。
type Options struct {
	File            string
	All             bool
	DB              bool
	Index           bool
	RBAC            bool
	RS              bool
	VideoPartitions bool
	Banner          bool
	SuperUser       bool
	// Args 保存 flag.Parse 之后剩余的位置参数（如 -su 的用户名与密码）。
	Args []string
}

// ParseOptions 解析命令行参数，返回实例交由 wire 注入。
func ParseOptions() *Options {
	opts := new(Options)

	flag.StringVar(&opts.File, "f", "config_dev.yaml", "配置文件")
	flag.BoolVar(&opts.All, "all", false, "初始化全部资源及超级管理员（db、vp、br、es、rs、rbac、su；可追加用户名和密码）")
	flag.BoolVar(&opts.DB, "db", false, "迁移数据库")
	flag.BoolVar(&opts.Index, "es", false, "es创建索引")
	flag.BoolVar(&opts.RBAC, "rbac", false, "初始化RBAC默认数据")
	flag.BoolVar(&opts.RS, "rs", false, "同步接口资源")
	flag.BoolVar(&opts.VideoPartitions, "vp", false, "初始化视频分区数据")
	flag.BoolVar(&opts.Banner, "br", false, "初始化Banner数据")
	flag.BoolVar(&opts.SuperUser, "su", false, "创建超级管理员账户")
	flag.Parse()

	opts.Args = flag.Args()

	return opts
}

// ProvideConfigPath 供 wire 使用：从命令行选项取配置文件路径。
func ProvideConfigPath(opts *Options) string {
	return opts.File
}
