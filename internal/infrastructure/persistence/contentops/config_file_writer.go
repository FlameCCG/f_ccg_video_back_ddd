package contentops

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/spf13/viper"
)

// ConfigFileWriter 把单个配置段写回 YAML 配置文件。
//
// 原实现内联在 api/site_api/admin_config.go：新建 viper 实例读入 global.ConfigPath 指向的文件，
// v.Set(name, item) 后 WriteConfig。现在保留配置段更新语义，使用同目录临时文件与原子替换，
// 文件路径在构造期注入（不再依赖全局变量）。
type ConfigFileWriter struct {
	path string
	mu   sync.Mutex
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

// Write 把 item 写入配置文件的 name 配置段。
//
// 读取失败与写入失败分别包装为 app.ErrConfigFileRead / app.ErrConfigFileWrite，
// 供 interfaces 层回显与原实现一致的错误文案。
func (w *ConfigFileWriter) Write(name string, item any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	v := viper.New()
	v.SetConfigFile(w.path)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileRead, err)
	}

	// 将内存中修改后的对应项设置进 viper
	v.Set(name, item)

	// Persist through a same-directory rename: readers never observe a partially written YAML file.
	raw, err := yaml.Marshal(v.AllSettings())
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}

	temp, err := os.CreateTemp(filepath.Dir(w.path), ".config-*")
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}
	defer func() { _ = os.Remove(temp.Name()) }()

	mode := os.FileMode(0o600)
	if stat, statErr := os.Stat(w.path); statErr == nil {
		mode = stat.Mode().Perm()
	}

	if err = temp.Chmod(mode); err == nil {
		_, err = temp.Write(raw)
	}

	if err == nil {
		err = temp.Sync()
	}

	closeErr := temp.Close()
	if err == nil {
		err = closeErr
	}

	if err == nil {
		err = os.Rename(temp.Name(), w.path)
	}

	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}

	return nil
}
