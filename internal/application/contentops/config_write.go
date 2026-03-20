package contentops

import (
	"fmt"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
)

// UpdateSite 更新站点配置段（空/脱敏的 MinIO 密钥保留原值）。
func (s *SiteConfigService) UpdateSite(siteCfg *site.Site, disableUserSubmission *bool) error {
	if siteCfg == nil {
		return fmt.Errorf("站点配置不能为空")
	}
	siteCfg = cloneAdminSiteConfig(siteCfg)
	return s.update(ConfigNameSite, func(next *config.Config) any {
		// 请求未提交投稿权限字段时保留当前策略，避免意外重新开放投稿。
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
		// 请求未携带本地消费开关时保留当前值，避免保存其他参数时重新启用本机消费。
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
