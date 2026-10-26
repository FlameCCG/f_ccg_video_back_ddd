package contentops

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	app "f_ccg_video_back_ddd/internal/application/contentops"

	"github.com/spf13/viper"
)

// ConfigFileWriter 把单个配置段写回 YAML 配置文件。
//
// 先写入同目录临时文件并同步，再原子替换，失败时保留旧文件。
type ConfigFileWriter struct {
	path string
	mu   sync.Mutex
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
	w.mu.Lock()
	defer w.mu.Unlock()
	v := viper.New()
	v.SetConfigFile(w.path)

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileRead, err)
	}

	// 将内存中修改后的对应项设置进 viper
	v.Set(name, item)

	info, err := os.Stat(w.path)
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileRead, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(w.path), ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}
	if err := v.WriteConfigAs(tmpPath); err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}
	f, err := os.OpenFile(tmpPath, os.O_RDWR, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}
	err = f.Chmod(info.Mode().Perm())
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, w.path)
	}
	if err != nil {
		return fmt.Errorf("%w: %w", app.ErrConfigFileWrite, err)
	}

	return nil
}
