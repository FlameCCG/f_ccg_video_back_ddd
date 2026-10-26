package contentops

import (
	"fmt"
	"net/url"
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"
)

// maskedSecretValue 管理端脱敏占位符：提交该值表示「不修改原密钥」。
const maskedSecretValue = "******"

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

// SiteConfigService 站点配置用例服务（原 api/site_api/admin_config.go 的配置读写逻辑）。
//
// 配置快照由注入的 shared.ConfigStore 端口持有（实现为 infraconfig.Provider，替代原 global.Config/SetConfig）：
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
// 管理端返回密钥明文（管理员需可直接查看/编辑），与原实现一致。
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

// UpdateSite 更新站点配置段（空/脱敏的 MinIO 密钥保留原值）。
func (s *SiteConfigService) UpdateSite(siteCfg *site.Site, disableUserSubmission *bool) error {
	if siteCfg == nil {
		return fmt.Errorf("站点配置不能为空")
	}
	siteCfg = cloneAdminSiteConfig(siteCfg)
	return s.update(ConfigNameSite, func(next *config.Config) any {
		// 旧管理端未提交新字段时保留当前策略，避免意外重新开放投稿。
		if disableUserSubmission != nil {
			siteCfg.DisableUserSubmission = *disableUserSubmission
		} else if next.Site != nil {
			siteCfg.DisableUserSubmission = next.Site.DisableUserSubmission
		}
		preserveSiteSecrets(siteCfg, next.Site)
		next.Site = siteCfg

		return next.Site
	})
}

// UpdateLogger 更新日志配置段。
func (s *SiteConfigService) UpdateLogger(logger *config.Logger) error {
	return s.update(ConfigNameLogger, func(next *config.Config) any {
		if logger != nil {
			nextLogger := *logger
			logger = &nextLogger
		}
		next.Logger = logger
		return next.Logger
	})
}

// UpdateEmail 更新邮件配置段（空/脱敏的授权码保留原值）。
func (s *SiteConfigService) UpdateEmail(email *config.Email) error {
	email = cloneAdminEmailConfig(email)
	return s.update(ConfigNameEmail, func(next *config.Config) any {
		preserveEmailSecrets(email, next.Email)
		next.Email = email

		return next.Email
	})
}

// UpdateAI 更新 AI 配置段（空/脱敏的模型密钥保留原值）。
func (s *SiteConfigService) UpdateAI(ai *config.AI) error {
	ai = cloneAdminAIConfig(ai)
	return s.update(ConfigNameAI, func(next *config.Config) any {
		preserveAISecrets(ai, next.AI)
		next.AI = ai

		return next.AI
	})
}

// UpdateTranscode 更新转码配置段；校验失败时不发布也不落盘。
func (s *SiteConfigService) UpdateTranscode(transcode *config.Transcode, localConsumerDisabled *bool) error {
	if transcode != nil {
		nextTranscode := *transcode
		nextTranscode.TranscodeResolutions = append([]uint16(nil), transcode.TranscodeResolutions...)
		transcode = &nextTranscode
	}
	return s.update(ConfigNameTranscode, func(next *config.Config) any {
		// 旧管理端未携带开关时保留本机角色，避免保存其他参数时重新启用本机消费。
		if transcode != nil {
			if localConsumerDisabled != nil {
				transcode.LocalConsumerDisabled = *localConsumerDisabled
			} else if next.Transcode != nil {
				transcode.LocalConsumerDisabled = next.Transcode.LocalConsumerDisabled
			}
		}
		next.Transcode = transcode

		return next.Transcode
	})
}

// UpdateThirdLogin 更新三方登录配置段（空/脱敏的 client secret 保留原值）。
func (s *SiteConfigService) UpdateThirdLogin(thirdLogin *config.ThirdLogin) error {
	thirdLogin = cloneAdminThirdLoginConfig(thirdLogin)
	return s.update(ConfigNameThirdLogin, func(next *config.Config) any {
		preserveThirdLoginSecrets(thirdLogin, next.ThirdLogin)
		next.ThirdLogin = thirdLogin

		return next.ThirdLogin
	})
}

// UpdateJWT 更新 JWT 配置段（空/脱敏的签名密钥保留原值）。
func (s *SiteConfigService) UpdateJWT(jwt *config.Jwt) error {
	jwt = cloneAdminJWTConfig(jwt)
	return s.update(ConfigNameJWT, func(next *config.Config) any {
		preserveJWTSecrets(jwt, next.Jwt)
		next.Jwt = jwt

		return next.Jwt
	})
}

// UpdateServerCORS 仅更新 server 段的 CORS Origin 白名单，保留 name/addr/mode。
//
// 写回 yaml 时使用完整 server 段，避免只写 corsOrigins 丢掉其它字段。
func (s *SiteConfigService) UpdateServerCORS(origins []string) error {
	normalized, err := normalizeAdminCorsOrigins(origins)
	if err != nil {
		return err
	}

	return s.update(ConfigNameServer, func(next *config.Config) any {
		if next.Server == nil {
			next.Server = &config.Server{}
		}

		// 浅拷贝后只改 CorsOrigins，保留 name/addr/mode。
		srv := *next.Server
		srv.CorsOrigins = normalized
		next.Server = &srv

		return next.Server
	})
}

// update 在持锁期间基于最新快照修改；所有校验与文件写入均先于发布。
func (s *SiteConfigService) update(name string, change func(*config.Config) any) error {
	return s.provider.Update(func(next *config.Config) error {
		item := change(next)
		switch name {
		case ConfigNameJWT:
			if err := config.ValidateJWTConfig(next.Jwt); err != nil {
				return err
			}
		case ConfigNameTranscode:
			if err := next.Transcode.Validate(); err != nil {
				return err
			}
		}
		return s.files.Write(name, item)
	})
}

// normalizeAdminCorsOrigins 校验并归一化 Origin 列表（允许 "*" 或完整 http(s) URL）。
func normalizeAdminCorsOrigins(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		if item == "*" {
			if _, ok := seen["*"]; !ok {
				seen["*"] = struct{}{}
				out = append(out, "*")
			}

			continue
		}

		parsed, err := url.Parse(item)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("无效的 CORS Origin: %s（请使用完整 URL，如 https://www.example.com）", item)
		}

		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, fmt.Errorf("无效的 CORS Origin scheme: %s（仅支持 http/https）", item)
		}

		// 规范化：去掉 path/query，只保留 scheme://host
		normalized := parsed.Scheme + "://" + parsed.Host
		if _, ok := seen[normalized]; ok {
			continue
		}

		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	return out, nil
}

// cloneAdminServerCORSConfig 提取管理端可见的 CORS 白名单（去空白、去空项）。
func cloneAdminServerCORSConfig(src *config.Server) AdminServerCORS {
	if src == nil {
		return AdminServerCORS{CorsOrigins: []string{}}
	}

	out := make([]string, 0, len(src.CorsOrigins))
	for _, o := range src.CorsOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}

	return AdminServerCORS{CorsOrigins: out}
}

// cloneSiteConfig 深拷贝站点配置并隐藏存储私有字段（本地路径、MinIO 凭据、分块目录）。
//
// 保留原实现的结构：先深拷贝 Local/MinIO，再在任一私有字段存在时整体清空。
// 由于三者全为零值时清空亦无副作用，等价于「一律隐藏私有存储字段」，
// 这里不做简化以保证与原实现逐行一致。
func cloneSiteConfig(src site.Site) site.Site {
	dst := src
	if src.Login != nil {
		login := *src.Login
		dst.Login = &login
	}

	if src.Register != nil {
		register := *src.Register
		dst.Register = &register
	}

	if src.ContentReview != nil {
		contentReview := *src.ContentReview
		dst.ContentReview = &contentReview
	}

	if src.Storage != nil {
		storage := *src.Storage
		if src.Storage.Local != nil {
			local := *src.Storage.Local
			storage.Local = &local
		}

		if src.Storage.MinIO != nil {
			minio := *src.Storage.MinIO
			storage.MinIO = &minio
		}

		if src.Storage.Local != nil || src.Storage.MinIO != nil || src.Storage.ChunkDir != "" {
			storage.Local = nil
			storage.MinIO = nil
			storage.ChunkDir = ""
		}
		// 前台按 chunkSize 切分。公开配置把它压到不超过 maxChunkSize，避免每片都超过服务端上限。
		if storage.MaxChunkSize > 0 && storage.ChunkSize > storage.MaxChunkSize {
			storage.ChunkSize = storage.MaxChunkSize
		}

		dst.Storage = &storage
	}

	return dst
}

// cloneAdminSiteConfig 返回管理端完整站点配置（密钥明文；管理员需可直接查看/编辑）。
func cloneAdminSiteConfig(src *site.Site) *site.Site {
	if src == nil {
		return nil
	}

	dst := cloneSiteConfig(*src)
	if src.Storage != nil {
		storage := *src.Storage
		if src.Storage.Local != nil {
			local := *src.Storage.Local
			storage.Local = &local
		}

		if src.Storage.MinIO != nil {
			minio := *src.Storage.MinIO
			storage.MinIO = &minio
		}

		dst.Storage = &storage
	}

	return &dst
}

// cloneAdminEmailConfig 浅拷贝邮件配置。
func cloneAdminEmailConfig(src *config.Email) *config.Email {
	if src == nil {
		return nil
	}

	dst := *src

	return &dst
}

// cloneAdminAIConfig 浅拷贝 AI 配置。
func cloneAdminAIConfig(src *config.AI) *config.AI {
	if src == nil {
		return nil
	}

	dst := *src

	return &dst
}

// cloneAdminJWTConfig 浅拷贝 JWT 配置。
func cloneAdminJWTConfig(src *config.Jwt) *config.Jwt {
	if src == nil {
		return nil
	}

	dst := *src

	return &dst
}

// cloneAdminThirdLoginConfig 深拷贝三方登录配置（逐个提供商单独复制）。
func cloneAdminThirdLoginConfig(src *config.ThirdLogin) *config.ThirdLogin {
	if src == nil {
		return nil
	}

	dst := *src
	if src.QQ != nil {
		qq := *src.QQ
		dst.QQ = &qq
	}

	if src.Google != nil {
		google := *src.Google
		dst.Google = &google
	}

	if src.GitHub != nil {
		github := *src.GitHub
		dst.GitHub = &github
	}

	if src.LinuxDo != nil {
		linuxDo := *src.LinuxDo
		dst.LinuxDo = &linuxDo
	}

	if src.X != nil {
		x := *src.X
		dst.X = &x
	}

	return &dst
}

// preserveSiteSecrets 保留站点配置中的 MinIO 密钥。
func preserveSiteSecrets(next, current *site.Site) {
	if next == nil || current == nil || next.Storage == nil || current.Storage == nil ||
		next.Storage.MinIO == nil || current.Storage.MinIO == nil {
		return
	}

	if shouldPreserveSecret(next.Storage.MinIO.SecretKey) {
		next.Storage.MinIO.SecretKey = current.Storage.MinIO.SecretKey
	}
}

// preserveEmailSecrets 保留邮件授权码。
func preserveEmailSecrets(next, current *config.Email) {
	if next == nil || current == nil {
		return
	}

	if shouldPreserveSecret(next.AuthCode) {
		next.AuthCode = current.AuthCode
	}
}

// preserveAISecrets 保留 AI 模型密钥。
func preserveAISecrets(next, current *config.AI) {
	if next == nil || current == nil {
		return
	}

	if shouldPreserveSecret(next.ChatModelAPIKey) {
		next.ChatModelAPIKey = current.ChatModelAPIKey
	}

	if shouldPreserveSecret(next.DoubaoModelAPIKey) {
		next.DoubaoModelAPIKey = current.DoubaoModelAPIKey
	}
}

// preserveJWTSecrets 保留 JWT 签名密钥。
func preserveJWTSecrets(next, current *config.Jwt) {
	if next == nil || current == nil {
		return
	}

	if shouldPreserveSecret(next.AccessSecret) {
		next.AccessSecret = current.AccessSecret
	}

	if shouldPreserveSecret(next.RefreshSecret) {
		next.RefreshSecret = current.RefreshSecret
	}
}

// preserveThirdLoginSecrets 保留各三方登录提供商的密钥。
func preserveThirdLoginSecrets(next, current *config.ThirdLogin) {
	if next == nil || current == nil {
		return
	}

	if next.QQ != nil && current.QQ != nil {
		preserveQQSecret(next.QQ, current.QQ)
	}

	if next.Google != nil && current.Google != nil && shouldPreserveSecret(next.Google.ClientSecret) {
		next.Google.ClientSecret = current.Google.ClientSecret
	}

	if next.GitHub != nil && current.GitHub != nil && shouldPreserveSecret(next.GitHub.ClientSecret) {
		next.GitHub.ClientSecret = current.GitHub.ClientSecret
	}

	if next.LinuxDo != nil && current.LinuxDo != nil && shouldPreserveSecret(next.LinuxDo.ClientSecret) {
		next.LinuxDo.ClientSecret = current.LinuxDo.ClientSecret
	}

	if next.X != nil && current.X != nil && shouldPreserveSecret(next.X.ClientSecret) {
		next.X.ClientSecret = current.X.ClientSecret
	}
}

// preserveQQSecret 保留 QQ 互联的 AppKey。
func preserveQQSecret(next, current *thirdlogin.QQ) {
	if shouldPreserveSecret(next.AppKey) {
		next.AppKey = current.AppKey
	}
}

// shouldPreserveSecret：管理端提交空字符串或历史脱敏占位符时，保留服务端原密钥，避免误覆盖。
func shouldPreserveSecret(value string) bool {
	trimmed := strings.TrimSpace(value)

	return trimmed == "" || trimmed == maskedSecretValue
}
