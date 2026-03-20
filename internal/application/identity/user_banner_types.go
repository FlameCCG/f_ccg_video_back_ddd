package identity

import (
	"io"
)

// UploadBannerInput 上传用户主页横幅入参。
type UploadBannerInput struct {
	// FileHash 前端计算的文件哈希，同时作为对象名。
	FileHash string
	// File 上传文件内容，必须可 Seek（图片嗅探需要回绕）。
	File io.ReadSeeker
	// Filename 原始文件名，用于扩展名校验。
	Filename string
	// Size 文件字节数。
	Size int64
}

// DefaultUserBannerList 系统默认用户主页横幅列表。
type DefaultUserBannerList struct {
	// BannerIDs 是系统配置中的默认横幅ID顺序。
	BannerIDs             []uint           `json:"bannerIds"`
	CurrentBannerID       uint             `json:"currentBannerId"`
	CurrentBannerImageURL string           `json:"currentBannerImageUrl"`
	List                  []UserBannerItem `json:"list"`
}

// AdminUpdateUserBannerListResult 管理端增删系统默认横幅后的结果。
type AdminUpdateUserBannerListResult struct {
	BannerIDs []uint `json:"bannerIds"`
}
