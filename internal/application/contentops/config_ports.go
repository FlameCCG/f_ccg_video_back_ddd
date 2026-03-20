package contentops

// ConfigFileWriter 配置文件写回端口。
//
// 管理端更新配置后需要把对应配置段持久化回 YAML 文件。
// 文件格式与序列化细节属于技术实现，落在 infrastructure；
// 实现须把读取失败与写入失败分别包装为 ErrConfigFileRead / ErrConfigFileWrite。
type ConfigFileWriter interface {
	// Write 把 item 写入配置文件的 name 配置段。
	Write(name string, item any) error
}
