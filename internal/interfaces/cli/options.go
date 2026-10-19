// Package cli 提供命令行入口相关的选项解析与运维任务（原 flags 包）。
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
