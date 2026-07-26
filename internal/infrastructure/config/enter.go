// Package config provides package functionality.
package config

import "f_ccg_video_back_ddd/internal/infrastructure/config/site"

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
