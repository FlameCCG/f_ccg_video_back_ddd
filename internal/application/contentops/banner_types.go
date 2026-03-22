package contentops

import (
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// VisibleBannerParams 前台可见轮播图查询参数。
type VisibleBannerParams struct {
	// Type 轮播图类型原始值；0 表示未指定。
	Type int
	// PartitionID 分区 ID；nil 表示未指定。
	PartitionID *uint
}

// AdminBannerListParams 后台轮播图列表查询参数。
type AdminBannerListParams struct {
	// Page 页码。
	Page int
	// PageSize 每页条数。
	PageSize int
	// Type 轮播图类型原始值；0 表示未指定。
	Type int
	// PartitionID 分区 ID；nil 表示不限分区。
	PartitionID *uint
	// Show 显示状态；nil 表示不限显示状态。
	Show *bool
}

// CreateBannerParams 创建轮播图参数。
type CreateBannerParams struct {
	// Cover 图片链接。
	Cover string
	// Href 跳转链接。
	Href string
	// Show 是否显示；nil 时默认显示。
	Show *bool
	// Type 轮播图类型原始值；0 时默认首页轮播图。
	Type int
	// PartitionID 分区 ID；nil 视为 0。
	PartitionID *uint
}

// UpdateBannerParams 更新轮播图参数；指针字段为 nil 表示不更新。
type UpdateBannerParams struct {
	// ID 轮播图主键。
	ID uint
	// Cover 图片链接。
	Cover *string
	// Href 跳转链接。
	Href *string
	// Show 是否显示。
	Show *bool
	// Type 轮播图类型原始值。
	Type *int
	// PartitionID 分区 ID。
	PartitionID *uint
}

// BannerDTO 是轮播图的 HTTP 响应和可见缓存 JSON 形状。
//
// 字段不使用 omitempty，零值也会输出；CreatedAt / UpdatedAt 不出现在 JSON 中。缓存按此结构解码。
type BannerDTO struct {
	ID          uint            `json:"id"`
	Cover       string          `json:"cover"`
	Href        string          `json:"href"`
	Show        bool            `json:"show"`
	Type        enum.BannerType `json:"type"`
	PartitionID uint            `json:"partitionId"`
}

// FromBanner 按 JSON 字段从实体组装响应或缓存 DTO。
func (BannerDTO) FromBanner(banner contentopsdomain.Banner) BannerDTO {
	return BannerDTO{
		ID:          banner.ID(),
		Cover:       banner.Cover(),
		Href:        banner.Href(),
		Show:        banner.Show(),
		Type:        banner.Type(),
		PartitionID: banner.PartitionID(),
	}
}

// FromBanners 批量组装 DTO。nil 保持 nil，空切片保持非 nil 空切片。
func (d BannerDTO) FromBanners(items []contentopsdomain.Banner) []BannerDTO {
	if items == nil {
		return nil
	}

	out := make([]BannerDTO, len(items))
	for i := range items {
		out[i] = d.FromBanner(items[i])
	}

	return out
}
