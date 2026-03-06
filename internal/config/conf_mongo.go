package config

import (
	"fmt"
	"net/url"
)

// Mongo 配置 Mongo URI 或分项连接参数，以及连接超时和连接池上限。
type Mongo struct {
	URI        string `mapstructure:"uri" json:"uri"`
	Host       string `mapstructure:"host" json:"host"`
	Port       int    `mapstructure:"port" json:"port"`
	Username   string `mapstructure:"username" json:"username"`
	Password   string `mapstructure:"password" json:"password"`
	DBName     string `mapstructure:"db" json:"db"`
	AuthSource string `mapstructure:"authSource" json:"authSource"`
	TimeoutSec int    `mapstructure:"timeoutSec" json:"timeoutSec"`
	MaxPool    uint64 `mapstructure:"maxPool" json:"maxPool"`
}

// GetURI 优先返回显式 URI，否则拼接分项参数；未指定主机/端口时使用本机默认值。
// 认证参数需按 URL 规则转义，调用方不应把返回连接串写入日志。
func (m Mongo) GetURI() string {
	if m.URI != "" {
		return m.URI
	}

	host := m.Host
	if host == "" {
		host = "127.0.0.1"
	}

	port := m.Port
	if port == 0 {
		port = 27017
	}

	if m.Username == "" {
		return fmt.Sprintf("mongodb://%s:%d", host, port)
	}

	authSource := m.AuthSource
	if authSource == "" {
		authSource = "admin"
	}

	return fmt.Sprintf(
		"mongodb://%s:%s@%s:%d/?authSource=%s",
		url.PathEscape(m.Username),
		url.PathEscape(m.Password),
		host,
		port,
		url.QueryEscape(authSource),
	)
}
