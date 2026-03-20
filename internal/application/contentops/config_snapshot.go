package contentops

import (
	"strings"

	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
)

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
// 先深拷贝 Local/MinIO，再清空私有存储字段。
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
