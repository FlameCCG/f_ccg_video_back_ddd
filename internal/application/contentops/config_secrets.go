package contentops

import (
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	thirdlogin "f_ccg_video_back_ddd/internal/config/thirdlogin"
)

// maskedSecretValue 管理端脱敏占位符：提交该值表示「不修改原密钥」。
const maskedSecretValue = "******"

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
