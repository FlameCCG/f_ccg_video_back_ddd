package config

import "fmt"

// Mysql 类型.
type Mysql struct {
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	Host     string `mapstructure:"host" json:"host"`
	Port     int    `mapstructure:"port" json:"port"`
	DBName   string `mapstructure:"db" json:"db"`
}

// GetDSN 方法.
func (d Mysql) GetDSN() string {
	// 参数插值把 prepare/execute/close 合并为一次往返；utf8mb4 不属于驱动禁止的危险字符集。
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&allowNativePasswords=true&interpolateParams=true&timeout=3s",
		d.Username, d.Password, d.Host, d.Port, d.DBName)
}
