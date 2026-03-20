package identity

// ConfigFileWriter 配置文件写回端口。
//
// 管理端改动系统默认横幅后需要把 site 配置段持久化回 YAML。
// 与 application/contentops.ConfigFileWriter 同签名，可绑定同一实现。
type ConfigFileWriter interface {
	// Write 把 item 写入配置文件的 name 配置段。
	Write(name string, item any) error
}
