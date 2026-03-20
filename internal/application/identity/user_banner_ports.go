package identity

import (
	"context"
	"io"
)

// UserBannerItem 用户主页横幅读模型。
type UserBannerItem struct {
	ID          uint   `json:"id"`
	Cover       string `json:"cover"`
	Href        string `json:"href"`
	Show        bool   `json:"show"`
	Type        int    `json:"type"`
	PartitionID uint   `json:"partitionId"`
}

// UserBannerCatalog 用户主页横幅目录查询端口。
type UserBannerCatalog interface {
	// FindVisible 读取指定 ID 的可见用户横幅；不存在时返回 (nil, nil)。
	FindVisible(ctx context.Context, bannerID uint) (*UserBannerItem, error)
	// ListVisible 按传入 ID 顺序读取可见用户横幅，缺失的跳过。
	ListVisible(ctx context.Context, bannerIDs []uint) ([]UserBannerItem, error)
}

// BannerStorage 用户主页横幅文件存储端口。
//
// 跨上下文依赖：应绑定到 media/storage 上下文。
type BannerStorage interface {
	// ValidateUploadID 校验前端传入的文件哈希是否是安全的上传标识。
	ValidateUploadID(uploadID string) error
	// EnsureReady 校验存储驱动配置；配置无效时返回「存储配置错误」。
	EnsureReady() error
	// SaveBanner 保存横幅文件并返回可访问 URL。
	SaveBanner(ctx context.Context, relativePath string, size int64, reader io.Reader, contentType string) (string, error)
}
