package contentops

import (
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

// bannerTypeOrZero 把请求中的类型原始值转为枚举；非正值表示未指定。
func bannerTypeOrZero(raw int) enum.BannerType {
	if raw > 0 {
		return enum.BannerType(raw)
	}

	return 0
}
