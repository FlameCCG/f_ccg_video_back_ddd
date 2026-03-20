package contentops

import (
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
)

// 可读写的配置项名称（= 查询参数 name，也是写回 YAML 时的配置段 key，属于外部契约）。
const (
	// ConfigNameSite 站点配置段.
	ConfigNameSite = "site"
	// ConfigNameLogger 日志配置段.
	ConfigNameLogger = "logger"
	// ConfigNameEmail 邮件配置段.
	ConfigNameEmail = "email"
	// ConfigNameAI AI 配置段.
	ConfigNameAI = "ai"
	// ConfigNameTranscode 转码配置段.
	ConfigNameTranscode = "transcode"
	// ConfigNameThirdLogin 三方登录配置段.
	ConfigNameThirdLogin = "thirdLogin"
	// ConfigNameServer 服务端配置段.
	ConfigNameServer = "server"
	// ConfigNameJWT JWT 配置段.
	ConfigNameJWT = "jwt"
)

// SiteConfigService 站点配置用例服务。
//
// 配置快照由注入的 shared.ConfigStore 持有，实现为 infraconfig.Provider。
// 更新与热更新共用串行化入口；校验并持久化成功后才发布快照。
type SiteConfigService struct {
	provider shared.ConfigStore
	files    ConfigFileWriter
}

// NewSiteConfigService 构造站点配置用例服务。
func NewSiteConfigService(provider shared.ConfigStore, files ConfigFileWriter) *SiteConfigService {
	return &SiteConfigService{provider: provider, files: files}
}

// PublicSite 返回面向普通用户的站点配置（隐藏存储私有字段）。
func (s *SiteConfigService) PublicSite() (site.Site, error) {
	cfg := s.provider.Get()
	if cfg == nil || cfg.Site == nil {
		return site.Site{}, ErrSiteConfigNotInitialized
	}

	return cloneSiteConfig(*cfg.Site), nil
}

// AdminConfigByName 按名称返回管理端可见的配置项；名称不支持时返回 ErrConfigItemUnsupported。
//
// 管理端返回密钥明文（管理员需可直接查看/编辑）。
func (s *SiteConfigService) AdminConfigByName(name string) (any, error) {
	cfg := s.provider.Get()
	if cfg == nil {
		cfg = &config.Config{}
	}

	switch name {
	case ConfigNameSite:
		return cloneAdminSiteConfig(cfg.Site), nil
	case ConfigNameEmail:
		return cloneAdminEmailConfig(cfg.Email), nil
	case ConfigNameAI:
		return cloneAdminAIConfig(cfg.AI), nil
	case ConfigNameThirdLogin:
		return cloneAdminThirdLoginConfig(cfg.ThirdLogin), nil
	case ConfigNameJWT:
		return cloneAdminJWTConfig(cfg.Jwt), nil
	case ConfigNameServer:
		return cloneAdminServerCORSConfig(cfg.Server), nil
	case ConfigNameLogger:
		return cfg.Logger, nil
	case ConfigNameTranscode:
		return cfg.Transcode, nil
	default:
		return nil, ErrConfigItemUnsupported
	}
}
