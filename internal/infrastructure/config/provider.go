// Package config 用 Viper 与 fsnotify 加载和热更新配置。
//
// Provider 用 atomic.Pointer 发布配置快照；加载与校验成功后才替换，热更新失败保留上一份有效配置。
// internal/config 定义数据形状，本包提供机制，bootstrap 将 Provider 绑定到应用层配置端口。
// narrow.go 把根配置收窄为连接构造所需的子配置；读取新快照不意味着现有连接自动重建。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package config

import (
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/pkg/logsafe"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	config        atomic.Pointer[config.Config]
	logger        atomic.Pointer[zap.Logger]
	path          string
	initialLogger *config.Logger
	mu            sync.Mutex
	subscribers   map[uint64]func(*config.Config)
	nextID        uint64
	watcher       *fsnotify.Watcher
	done          chan struct{}
	stopOnce      sync.Once
	watchWG       sync.WaitGroup
}

// NewProvider 用给定配置快照构造 Provider（测试与 CLI 可直接使用）。
func NewProvider(c *config.Config) *Provider {
	p := new(Provider)

	if c != nil && c.Logger != nil {
		copyCfg := *c.Logger
		p.initialLogger = &copyCfg
	}

	p.SetLogger(nil)
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

	p.publishLocked(c)
}

// Path 返回配置文件路径（管理端写回配置时使用）。
func (p *Provider) Path() string {
	if p == nil {
		return ""
	}

	return p.path
}

// Subscribe shares one notification path for file reloads and management updates.
// Unsubscribe waits for an in-flight callback before it returns.
func (p *Provider) Subscribe(fn func(*config.Config)) func() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.subscribers == nil {
		p.subscribers = make(map[uint64]func(*config.Config))
	}

	p.nextID++
	id := p.nextID
	p.subscribers[id] = fn
	fn(p.Get())

	return func() { p.mu.Lock(); defer p.mu.Unlock(); delete(p.subscribers, id) }
}

// Close stops the file watcher before the logger is closed.
func (p *Provider) Close() {
	if p == nil {
		return
	}

	p.stopOnce.Do(func() {
		if p.done != nil {
			close(p.done)
		}

		if p.watcher != nil {
			_ = p.watcher.Close()
		}

		p.watchWG.Wait()
	})
}

func readConfig(filePath string) (*config.Config, error) {
	v := viper.New()
	for _, scene := range []string{"login", "register"} {
		v.SetDefault("site."+scene+".slideCaptchaTTL", 300)
		v.SetDefault("site."+scene+".slideCaptchaPadding", 10)
		v.SetDefault("site."+scene+".textClickCaptchaTTL", 300)
		v.SetDefault("site."+scene+".textClickCaptchaPadding", 20)
	}

	v.SetConfigFile(filePath)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	c := new(config.Config)
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	c.Normalize()

	if err := config.ValidateRequired(c); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return c, nil
}

// Load validates before publication; predictable startup failures return to the entrypoint.
func Load(filePath string) (*Provider, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	c, err := readConfig(abs)
	if err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create config watcher: %w", err)
	}

	if err = watcher.Add(filepath.Dir(abs)); err != nil {
		_ = watcher.Close()
		return nil, fmt.Errorf("watch config: %w", err)
	}

	p := NewProvider(c)
	p.path, p.watcher, p.done = abs, watcher, make(chan struct{})

	p.watchWG.Add(1)
	go p.watch()

	return p, nil
}

func (p *Provider) watch() {
	defer p.watchWG.Done()

	var (
		timer *time.Timer
		tick  <-chan time.Time
	)

	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-p.done:
			return
		case event, ok := <-p.watcher.Events:
			if !ok {
				return
			}

			if filepath.Clean(event.Name) != p.path || event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}

			timer = resetConfigDebounce(timer)

			tick = timer.C
		case err, ok := <-p.watcher.Errors:
			if !ok {
				return
			}

			p.log().Error("config.watch.failed", zap.String("event", "config.provider.watch.failed"), zap.String("operation", "config.provider.watch"), zap.String("error", logsafe.Text(err.Error(), logsafe.ErrorLimit)))
		case <-tick:
			tick = nil

			p.mu.Lock()

			next, err := readConfig(p.path)
			if err != nil {
				p.mu.Unlock()
				p.log().Error("config.reload.failed", zap.String("event", "config.provider.watch.failed"), zap.String("operation", "config.provider.watch"), zap.String("error", logsafe.Text(err.Error(), logsafe.ErrorLimit)))

				continue
			}

			p.publishLocked(next)
			p.mu.Unlock()
			p.log().Info("config.reload.completed", zap.String("event", "config.reload.completed"))
		}
	}
}

func (p *Provider) publishLocked(c *config.Config) {
	p.config.Store(c)

	for _, notify := range p.subscribers {
		notify(c)
	}
}

// PublishSection serializes persistence, file reloads and snapshot publication.
// It merges the requested section into the latest snapshot, avoiding lost concurrent updates.
func (p *Provider) PublishSection(name string, item any, persist func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	next := config.Clone(p.Get())
	tree := reflect.ValueOf(next).Elem()
	found := false

	for i := range tree.NumField() {
		if !strings.EqualFold(tree.Type().Field(i).Tag.Get("mapstructure"), name) {
			continue
		}

		value := reflect.ValueOf(item)
		switch {
		case !value.IsValid():
			tree.Field(i).SetZero()
		case value.Type().AssignableTo(tree.Field(i).Type()):
			tree.Field(i).Set(value)
		default:
			return errors.New("invalid config section type")
		}

		found = true

		break
	}

	if !found {
		return errors.New("unsupported config section")
	}

	if name == "logger" {
		if err := next.Logger.Validate(); err != nil {
			return err
		}
	}

	if err := persist(); err != nil {
		return err
	}

	p.publishLocked(next)

	return nil
}

// Provide gives every Wire graph a watcher cleanup, including failed construction and CLI tasks.
func Provide(path string) (*Provider, func(), error) {
	p, err := Load(path)
	if err != nil {
		return nil, nil, err
	}

	return p, p.Close, nil
}

// LoggerRestartRequired compares desired output settings with the startup snapshot.
func (p *Provider) LoggerRestartRequired(next *config.Logger) bool {
	return config.LoggerRestartRequired(p.initialLogger, next)
}

func resetConfigDebounce(timer *time.Timer) *time.Timer {
	if timer == nil {
		return time.NewTimer(100 * time.Millisecond)
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	timer.Reset(100 * time.Millisecond)

	return timer
}
