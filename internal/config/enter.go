// Package config 声明配置树的数据契约（共享内核）。
//
// 它是跨层的发布语言：结构体与 json/mapstructure tag 对齐外部契约
// （YAML 文件与管理端配置接口），供 application / interfaces / infrastructure
// 三层共同引用；本包只含纯数据与纯校验，零第三方依赖。
// 领域层（internal/domain/**）依旧禁止 import 本包；
// 配置的加载与热更新机制（viper/fsnotify）见 internal/infrastructure/config（见 D-020）。
package config

import "f_ccg_video_back_ddd/internal/config/site"

// Config 映射 config_dev.yaml 的整体结构.
type Config struct {
	Server     *Server     `mapstructure:"server" json:"server"`
	Logger     *Logger     `mapstructure:"logger" json:"logger"`
	Mysql      *Mysql      `mapstructure:"mysql" json:"mysql"`
	Redis      *Redis      `mapstructure:"redis" json:"redis"`
	Mongo      *Mongo      `mapstructure:"mongo" json:"mongo"`
	Jwt        *Jwt        `mapstructure:"jwt" json:"jwt"`
	Site       *site.Site  `mapstructure:"site" json:"site"`
	Email      *Email      `mapstructure:"email" json:"email"`
	AI         *AI         `mapstructure:"ai" json:"ai"`
	ThirdLogin *ThirdLogin `mapstructure:"thirdLogin" json:"thirdLogin"`
	ES         *ES         `mapstructure:"es" json:"es"`
	River      *River      `mapstructure:"river" json:"river"`
	Kafka      *Kafka      `mapstructure:"kafka" json:"kafka"`
	Transcode  *Transcode  `mapstructure:"transcode" json:"transcode"`
}

// Normalize 方法.
func (c *Config) Normalize() {
	if c == nil {
		return
	}
}

// Clone 浅拷贝顶层 Config（嵌套指针共享）。
// 用于在不改动旧快照的前提下替换某个配置段。
func Clone(src *Config) *Config {
	if src == nil {
		return &Config{}
	}

	dst := *src

	return &dst
}
