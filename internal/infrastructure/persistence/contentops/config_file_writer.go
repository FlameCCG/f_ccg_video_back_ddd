package contentops

import (
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	"github.com/spf13/viper"
)

// ConfigFileWriter 把单个配置段写回 YAML 配置文件。
//
// 原实现内联在 api/site_api/admin_config.go：新建 viper 实例读入 global.ConfigPath 指向的文件，
// v.Set(name, item) 后 WriteConfig。这里保持完全相同的读—改—写序列与序列化行为，
// 只是把文件路径改为构造期注入（不再依赖全局变量）。
type ConfigFileWriter struct {
	path string
}

var _ app.ConfigFileWriter = (*ConfigFileWriter)(nil)

// NewConfigFileWriter 用显式配置文件路径构造写回器（测试可指向临时文件）。
func NewConfigFileWriter(path string) *ConfigFileWriter {
	return &ConfigFileWriter{path: path}
}

// Write 把 item 写入配置文件的 name 配置段。
//
// 读取失败与写入失败分别包装为 app.ErrConfigFileRead / app.ErrConfigFileWrite，
// 供 interfaces 层回显与原实现一致的错误文案。
func (w *ConfigFileWriter) Write(name string, item any) error {
	v := viper.New()
	v.SetConfigFile(w.path)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileRead, err)
	}

	// 将内存中修改后的对应项设置进 viper
	v.Set(name, item)

	// 写入文件
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}

	return nil
}
