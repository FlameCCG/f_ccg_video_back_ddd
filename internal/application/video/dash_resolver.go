package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// DASHVariantResolver 解析 DASH 资源的 MPD 清单，展开 Representation 列表。
type DASHVariantResolver interface {
	// Variants 拉取并解析 MPD；失败时返回错误（调用方按最佳努力忽略）。
	Variants(ctx context.Context, resourceID uint, mpdURL string) ([]domain.DASHVariant, error)
}
