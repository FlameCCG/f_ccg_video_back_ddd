package config

import (
	"fmt"
	"net/url"
)

// Mongo 类型.
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

// GetURI 方法.
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
