package config

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// minJWTSecretLen JWT 签名密钥最小长度，防止弱密钥与空密钥进入运行时。
const minJWTSecretLen = 32

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
	config atomic.Pointer[Config]
	logger atomic.Pointer[zap.Logger]
	path   string
}

// NewProvider 用给定配置快照构造 Provider（测试与 CLI 可直接使用）。
func NewProvider(c *Config) *Provider {
	p := new(Provider)
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
func (p *Provider) Get() *Config {
	if p == nil {
		return nil
	}

	return p.config.Load()
}

// Set 原子发布新的配置树（初始化、热更新、管理端写配置）。
// 已发布的配置树应视为只读，更新时请浅拷贝顶层后再 Set。
func (p *Provider) Set(c *Config) {
	if p == nil {
		return
	}
	p.config.Store(c)
}

// Path 返回配置文件路径（管理端写回配置时使用）。
func (p *Provider) Path() string {
	if p == nil {
		return ""
	}

	return p.path
}

// Clone 浅拷贝顶层 Config（嵌套指针共享）。
// 用于在不改动旧快照的前提下替换某个配置段。
func Clone(src *Config) *Config {
	if src == nil {
		return &Config{}
	}

	dst := *src

	return &dst
}

// Load 读取配置文件、校验必填项并开启热更新监听，返回可注入的 Provider。
//
// 语义与原 core.InitConf 完全一致：读取失败/映射失败/校验失败均 panic；
// 热更新读取或校验失败时保留上一份有效配置并打日志。
func Load(filePath string) (*Provider, error) {
	v := viper.New()
	v.SetConfigFile(filePath)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	// 映射到结构体
	c := new(Config)
	if err := v.Unmarshal(c); err != nil {
		panic(fmt.Errorf("config unmarshal error: %w", err))
	}

	c.Normalize()

	if err := ValidateRequired(c); err != nil {
		panic(fmt.Errorf("config validation error: %w", err))
	}

	provider := NewProvider(c)
	provider.path = filePath

	// 开启热更新监听
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		// 稍微等待一下，确保文件写入完成
		time.Sleep(500 * time.Millisecond)

		provider.log().Info("配置文件热更新", zap.String("filename", e.Name))

		// 显式重新读取配置，防止 viper 内部缓存未更新或读取到空文件
		if err := v.ReadInConfig(); err != nil {
			provider.log().Error("配置文件热更新读取失败", zap.Error(err))

			return
		}

		// 重新读取配置
		newConf := new(Config)
		if err := v.Unmarshal(newConf); err != nil {
			provider.log().Error("配置文件热更新失败", zap.Error(err))

			return
		}

		newConf.Normalize()

		if err := ValidateRequired(newConf); err != nil {
			provider.log().Error("配置文件热更新校验失败，继续使用上一份有效配置", zap.Error(err))

			return
		}
		provider.Set(newConf)
		provider.log().Info("配置文件热更新成功!")
	})
	return provider, nil
}

// ValidateRequired 校验启动必须存在的配置段。
func ValidateRequired(c *Config) error {
	if c == nil {
		return errors.New("config is required")
	}

	if c.Transcode == nil {
		return errors.New("transcode config is required")
	}

	if err := validateJWTConfig(c.Jwt); err != nil {
		return err
	}

	return nil
}

func validateJWTConfig(j *Jwt) error {
	if j == nil {
		return errors.New("jwt config is required")
	}

	if err := validateJWTSecret("jwt.accessSecret", j.AccessSecret); err != nil {
		return err
	}

	if err := validateJWTSecret("jwt.refreshSecret", j.RefreshSecret); err != nil {
		return err
	}

	return nil
}

func validateJWTSecret(name, secret string) error {
	if strings.TrimSpace(secret) == "" {
		return fmt.Errorf("%s is required and must not be empty", name)
	}

	if len(secret) < minJWTSecretLen {
		return fmt.Errorf("%s must be at least %d characters", name, minJWTSecretLen)
	}

	return nil
}
