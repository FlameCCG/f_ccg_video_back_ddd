// Package config 用 Viper 与 fsnotify 加载和热更新配置。
//
// Provider 用 atomic.Pointer 发布配置快照；加载与校验成功后才替换，热更新失败保留上一份有效配置。
// internal/config 定义数据形状，本包提供机制，bootstrap 将 Provider 绑定到应用层配置端口。
// narrow.go 把根配置收窄为连接构造所需的子配置；读取新快照不意味着现有连接自动重建。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"f_ccg_video_back_ddd/internal/config"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Provider 持有当前生效的配置快照，并支持热更新原子替换。
//
// 它替代原 global 包中的 atomic.Pointer[conf.Config]：实例由 bootstrap 通过 wire 注入，
// 不再是包级可变单例。热更新会整体替换配置树；调用方应先取本地变量再读字段，
// 避免同请求内跨版本撕裂：
//
//	cfg := provider.Get()
//	if cfg == nil { ... }
//	use cfg.Site / cfg.Jwt ...
type Provider struct {
	mu     sync.Mutex
	config atomic.Pointer[config.Config]
	logger atomic.Pointer[zap.Logger]
	path   string
}

// NewProvider 用给定配置快照构造 Provider（测试与 CLI 可直接使用）。
func NewProvider(c *config.Config) *Provider {
	p := new(Provider)
	p.Set(c)
	return p
}

// SetLogger 原子替换热更新监听使用的日志器；nil 会归一化为无输出日志器。
// 配置必须先于正式日志器加载，因此由组合根在日志器构造完成后显式接入。
func (p *Provider) SetLogger(logger *zap.Logger) {
	if p == nil {
		return
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	p.logger.Store(logger)
}

func (p *Provider) log() *zap.Logger {
	if p == nil {
		return zap.NewNop()
	}

	logger := p.logger.Load()
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

// Get 返回当前配置快照指针（可能为 nil）。
//
// 原子读取只保护顶层指针，不深拷贝配置树；调用方应把返回对象当作只读，不能原地修改嵌套字段。
func (p *Provider) Get() *config.Config {
	if p == nil {
		return nil
	}

	return p.config.Load()
}

// Set 原子发布新的配置树（初始化、热更新、管理端写配置）。
// 已发布的树必须视为只读。更新应创建新顶层，并拷贝所有将被修改的嵌套对象、map 或 slice；
// 仅浅拷贝顶层不能隔离共享子对象。Set 本身不校验数据，也不写文件。
func (p *Provider) Set(c *config.Config) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.Store(c)
}

// Update 将读、改、持久化和发布放在同一临界区，与热更新共用锁。
func (p *Provider) Update(change func(*config.Config) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	next := config.Clone(p.Get())
	if err := change(next); err != nil {
		return err
	}
	p.config.Store(next)
	return nil
}

// Path 返回配置文件路径（管理端写回配置时使用）。
func (p *Provider) Path() string {
	if p == nil {
		return ""
	}

	return p.path
}

// Load 读取配置文件、校验必填项并开启热更新监听，返回可注入的 Provider。
//
// 读取、映射或校验失败时返回错误；热更新失败时保留上一份有效配置并打日志。
func Load(filePath string) (*Provider, error) {
	return load(filePath, true, config.ValidateRequired)
}

// LoadOnce 为一次性运维任务加载配置，不创建热更新监听。
func LoadOnce(filePath string) (*Provider, error) {
	return load(filePath, false, config.ValidateMaintenance)
}

// LoadTranscodeWorker uses the same role validation on initial load and reload.
func LoadTranscodeWorker(filePath string) (*Provider, error) {
	return load(filePath, true, config.ValidateTranscodeWorker)
}

func load(filePath string, watch bool, validate func(*config.Config) error) (*Provider, error) {
	v := viper.New()
	// 缺少新增字段时使用独立默认值；显式配置的 0 边距仍会保留。
	for _, scene := range []string{"login", "register"} {
		v.SetDefault("site."+scene+".slideCaptchaTTL", 300)
		v.SetDefault("site."+scene+".slideCaptchaPadding", 10)
		v.SetDefault("site."+scene+".textClickCaptchaTTL", 300)
		v.SetDefault("site."+scene+".textClickCaptchaPadding", 20)
	}
	v.SetConfigFile(filePath)
	// 设置环境变量键名替换规则，最好用大写的环境变量名（linux区分大小写），比如 SITE_CAPTCHA_TTL
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// 映射到结构体
	c := new(config.Config)
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("config unmarshal error: %w", err)
	}

	if err := validate(c); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	provider := NewProvider(c)
	provider.path = filePath
	if !watch {
		return provider, nil
	}

	if err := provider.watchConfig(v, validate); err != nil {
		return nil, err
	}
	return provider, nil
}

// watchConfig 监听父目录，配置文件被替换或删除时仍继续监听后续重建。
// Windows 原子替换会先发出 Remove；Viper 的内置监听会在该事件上退出。
func (p *Provider) watchConfig(v *viper.Viper, validate func(*config.Config) error) error {
	filename, err := filepath.Abs(p.path)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	directory := filepath.Dir(filename)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create config watcher: %w", err)
	}
	if err := watcher.Add(directory); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("watch config directory: %w", err)
	}
	realFilename, _ := filepath.EvalSymlinks(filename)
	go func() {
		defer watcher.Close()
		var timer *time.Timer
		var reload <-chan time.Time
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				path := filepath.Clean(event.Name)
				if path == directory && event.Has(fsnotify.Remove|fsnotify.Rename) {
					return
				}
				currentFilename, _ := filepath.EvalSymlinks(filename)
				targetChanged := currentFilename != "" && currentFilename != realFilename
				if !targetChanged && (path != filename || !event.Has(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename)) {
					continue
				}
				if currentFilename != "" {
					realFilename = currentFilename
				}
				// 合并替换产生的一组事件，待文件写完后再读取。
				if timer == nil {
					timer = time.NewTimer(500 * time.Millisecond)
				} else {
					timer.Reset(500 * time.Millisecond)
				}
				reload = timer.C
			case <-reload:
				reload = nil
				p.reloadConfig(v, validate)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				p.log().Error("配置文件监听失败", zap.Error(err))
			}
		}
	}()
	return nil
}

func (p *Provider) reloadConfig(v *viper.Viper, validate func(*config.Config) error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log().Info("配置文件热更新", zap.String("filename", p.path))
	if err := v.ReadInConfig(); err != nil {
		p.log().Error("配置文件热更新读取失败", zap.Error(err))
		return
	}
	newConf := new(config.Config)
	if err := v.Unmarshal(newConf); err != nil {
		p.log().Error("配置文件热更新失败", zap.Error(err))
		return
	}
	if err := validate(newConf); err != nil {
		p.log().Error("配置文件热更新校验失败，继续使用上一份有效配置", zap.Error(err))
		return
	}
	p.config.Store(newConf)
	p.log().Info("配置文件热更新成功!")
}
