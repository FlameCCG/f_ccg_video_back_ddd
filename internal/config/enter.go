// Package config 定义配置树的数据契约与纯校验，作为应用、接口和基础设施共享的配置语言。
//
// 本包只保存结构体、标签及校验逻辑；文件读取、环境变量覆盖、监听与快照发布在 infrastructure/config。
// mapstructure 对应配置读取键，json 对应管理接口字段，两者与 config.example.yaml 需要一同理解。
// 领域层不依赖配置树：应用层先取出审核开关、策略或时间等值，再传给纯领域规则。
// 应用层经 application/shared.ConfigReader/ConfigStore 取得配置，避免直接耦合 Viper 的运行机制。
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

// Normalize 保留统一配置归一化入口；当前实现除 nil 检查外不修改字段，默认值主要由加载器和读取方提供。
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
