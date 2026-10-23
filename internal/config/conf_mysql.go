package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// DefaultMysqlTimezone 是 MySQL DATETIME/TIMESTAMP 的默认业务时区。
// 项目的 MySQL 与部署环境均使用 Asia/Shanghai，显式固定后不再受应用节点 time.Local 影响。
const DefaultMysqlTimezone = "Asia/Shanghai"

// Mysql 描述 MySQL 连接与业务时间解释方式；Timezone 也供 binlog 时间转换使用。
type Mysql struct {
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	DBName   string `mapstructure:"db" json:"db"`
	Timezone string `mapstructure:"timezone" json:"timezone"`
}

// TimezoneName 返回显式配置或项目默认的 MySQL 时区名。
func (d Mysql) TimezoneName() string {
	if timezone := strings.TrimSpace(d.Timezone); timezone != "" {
		return timezone
	}

	return DefaultMysqlTimezone
}

// LoadLocation 加载 MySQL 业务时区，供驱动和 binlog 转换共用。
func (d Mysql) LoadLocation() (*time.Location, error) {
	location, err := time.LoadLocation(d.TimezoneName())
	if err != nil {
		return nil, fmt.Errorf("load mysql timezone %q: %w", d.TimezoneName(), err)
	}

	return location, nil
}

// GetDSN 构造启用 parseTime 的 MySQL DSN，并显式设置时间解析的 loc。
// loc 决定驱动如何解释时间，不等于修改 MySQL 服务端的 session time_zone。
func (d Mysql) GetDSN() string {
	// 参数插值把 prepare/execute/close 合并为一次往返；utf8mb4 不属于驱动禁止的危险字符集。
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=%s&allowNativePasswords=true&interpolateParams=true&timeout=3s",
		d.Username, d.Password, d.Host, d.Port, d.DBName, url.QueryEscape(d.TimezoneName()))
}
