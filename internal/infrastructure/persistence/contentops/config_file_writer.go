package contentops

import (
	"fmt"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/spf13/viper"
)

// ConfigFileWriter 将单个配置段写回 YAML 文件。
//
// 每次构造独立 viper 实例读取指定路径，设置 name 配置段后写回，文件路径由构造函数注入。
type ConfigFileWriter struct {
	path string
}

var _ app.ConfigFileWriter = (*ConfigFileWriter)(nil)

// NewConfigFileWriter 用显式配置文件路径构造写回器（测试可指向临时文件）。
func NewConfigFileWriter(path string) *ConfigFileWriter {
	return &ConfigFileWriter{path: path}
}

// ProvideConfigFileWriter 供 wire 使用：从配置 Provider 取配置文件路径。
func ProvideConfigFileWriter(provider *infraconfig.Provider) *ConfigFileWriter {
	return NewConfigFileWriter(provider.Path())
}

// Write 将 item 写入配置文件的 name 段。
//
// 读取与写入失败分别包装为 app.ErrConfigFileRead 和 app.ErrConfigFileWrite，
// 供接口层映射用户可读文案。
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
