// Package contentops 是内容运营上下文（站点配置 + 站点统计 + 轮播图）的领域层。
//
// 本包为纯 Go 实现：不 import gin / gorm / redis / viper / zap，
// 只承载轮播图的字段裁定规则与站点统计的计算规则。
package contentops

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Banner 轮播图实体（原 models/mysqlEntity.Banner 的领域侧对应物）。
//
// 字段顺序与 json tag 与原实体逐字一致：该结构既直接作为 HTTP 响应体序列化，
// 也作为可见轮播图 Redis 缓存的序列化格式，两者都属于外部契约。
// CreatedAt / UpdatedAt 与原实体一样标记为 json:"-"，不对外暴露。
type Banner struct {
	ID          uint            `json:"id"`
	CreatedAt   time.Time       `json:"-"`
	UpdatedAt   time.Time       `json:"-"`
	Cover       string          `json:"cover"`
	Href        string          `json:"href"`
	Show        bool            `json:"show"`
	Type        enum.BannerType `json:"type"`
	PartitionID uint            `json:"partitionId"`
}

// BannerChanges 轮播图部分字段更新集合；字段为 nil 表示本次不更新该字段。
//
// 对应原实现中由 api 层拼装、再传给 banner_service.Update 的 map[string]any；
// 新架构不让列名离开持久化层，改由仓储实现把本结构翻译为 GORM 更新集。
type BannerChanges struct {
	Cover       *string
	Href        *string
	Show        *bool
	Type        *enum.BannerType
	PartitionID *uint
}

// IsEmpty 报告是否没有任何待更新字段。
func (c BannerChanges) IsEmpty() bool {
	return c.Cover == nil && c.Href == nil && c.Show == nil && c.Type == nil && c.PartitionID == nil
}

// BannerAdminQuery 后台轮播图分页查询条件。
type BannerAdminQuery struct {
	// Page 页码，从 1 开始。
	Page int
	// PageSize 每页条数。
	PageSize int
	// Type 轮播图类型；0 表示不限类型。
	Type enum.BannerType
	// PartitionID 分区 ID；nil 表示不限分区。
	PartitionID *uint
	// Show 显示状态；nil 表示不限显示状态。
	Show *bool
}

// 后台分页默认值（与原 banner_service.ListAdmin 一致）。
const (
	bannerAdminDefaultPageSize = 20
	bannerAdminMaxPageSize     = 100
)

// Normalize 归一化分页参数：页码非正回落 1，每页条数非正或超过 100 回落 20。
func (q *BannerAdminQuery) Normalize() {
	if q == nil {
		return
	}

	if q.Page <= 0 {
		q.Page = 1
	}

	if q.PageSize <= 0 || q.PageSize > bannerAdminMaxPageSize {
		q.PageSize = bannerAdminDefaultPageSize
	}
}

// ResolveBannerPartitionID 依据轮播图类型裁定分区 ID。
//
// 规则与原 api/banner_api/helper.go 的 normalizeBannerPartitionID 一致：
//   - 非首页类型不允许携带分区 ID（携带即报 ErrBannerPartitionTypeUnsupported），一律归零；
//   - 首页类型的 0 表示首页本身，无需校验分区是否存在；
//   - 首页类型的非 0 分区需要调用方另行校验分区存在性，此时 needExists 为 true。
//
// 领域层只裁定规则，分区存在性属于 IO，由应用层经查询端口完成。
func ResolveBannerPartitionID(bannerType enum.BannerType, partitionID uint) (resolved uint, needExists bool, err error) {
	if bannerType != enum.BannerTypeHome {
		if partitionID > 0 {
			return 0, false, ErrBannerPartitionTypeUnsupported
		}

		return 0, false, nil
	}

	if partitionID == 0 {
		return 0, false, nil
	}

	return partitionID, true, nil
}
