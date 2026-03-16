package config

import (
	"net"
	"net/url"
	"strconv"
)

// Mongo 用分项连接参数描述 MongoDB，并包含连接超时和连接池上限。
type Mongo struct {
	ReplicaSet       string `mapstructure:"replicaSet" json:"replicaSet"`
	DirectConnection bool   `mapstructure:"directConnection" json:"directConnection"`
	Host             string `mapstructure:"host" json:"host"`
	Port             int    `mapstructure:"port" json:"port"`
	Username         string `mapstructure:"username" json:"username"`
	Password         string `mapstructure:"password" json:"password"`
	DBName           string `mapstructure:"db" json:"db"`
	AuthSource       string `mapstructure:"authSource" json:"authSource"`
	TimeoutSec       int    `mapstructure:"timeoutSec" json:"timeoutSec"`
	MaxPool          uint64 `mapstructure:"maxPool" json:"maxPool"`
}

// GetURI 由主机、端口和账号拼接 Mongo 连接串。未指定主机或端口时使用本机默认值。
// 账号和密码按 userinfo 规则转义，调用方不应把返回连接串写入日志。
func (m Mongo) GetURI() string {
	host := m.Host
	if host == "" {
		host = "127.0.0.1"
	}

	port := m.Port
	if port == 0 {
		port = 27017
	}

	u := url.URL{
		Scheme: "mongodb",
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	query := u.Query()
	if m.ReplicaSet != "" {
		query.Set("replicaSet", m.ReplicaSet)
	}
	if m.DirectConnection {
		query.Set("directConnection", "true")
	}
	if m.Username != "" {
		authSource := m.AuthSource
		if authSource == "" {
			authSource = "admin"
		}
		u.User = url.UserPassword(m.Username, m.Password)
		u.Path = "/"
		query.Set("authSource", authSource)
	}
	u.RawQuery = query.Encode()

	return u.String()
}
