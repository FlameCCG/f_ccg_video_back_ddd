package contentops

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Banner 轮播图实体，状态字段不导出。
//
// HTTP 响应和可见缓存使用 DTO，零值字段也输出；CreatedAt / UpdatedAt 不出现在 JSON 中。
type Banner struct {
	id          uint
	createdAt   time.Time
	updatedAt   time.Time
	cover       string
	href        string
	show        bool
	bannerType  enum.BannerType
	partitionID uint
}

// BannerSnapshot 是轮播图的持久化快照。字段公开，供基础设施重建历史数据。
type BannerSnapshot struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Cover       string
	Href        string
	Show        bool
	Type        enum.BannerType
	PartitionID uint
}

// NewBanner 按现有创建默认值构造轮播图：类型先落在首页，不额外校验封面或分区。
// 类型与分区的组合由 ChangePlacement 裁定。
func NewBanner(cover, href string, show bool) *Banner {
	return &Banner{
		cover:      cover,
		href:       href,
		show:       show,
		bannerType: enum.BannerTypeHome,
	}
}

// RestoreBanner 从持久化快照恢复轮播图，不重新执行创建或分区校验。
func RestoreBanner(snapshot BannerSnapshot) *Banner {
	return &Banner{
		id:          snapshot.ID,
		createdAt:   snapshot.CreatedAt,
		updatedAt:   snapshot.UpdatedAt,
		cover:       snapshot.Cover,
		href:        snapshot.Href,
		show:        snapshot.Show,
		bannerType:  snapshot.Type,
		partitionID: snapshot.PartitionID,
	}
}

// Snapshot 返回当前状态的副本。修改返回值不会影响实体。
func (b Banner) Snapshot() BannerSnapshot {
	return BannerSnapshot{
		ID:          b.id,
		CreatedAt:   b.createdAt,
		UpdatedAt:   b.updatedAt,
		Cover:       b.cover,
		Href:        b.href,
		Show:        b.show,
		Type:        b.bannerType,
		PartitionID: b.partitionID,
	}
}

// ID 返回主键。
func (b Banner) ID() uint { return b.id }

func (c BannerChanges) ValidateFor(id uint) error {
	if id == 0 {
		return ErrBannerInvalidParam
	}
	if c.IsEmpty() {
		return ErrBannerNoUpdatableField
	}
	return nil
}

// CreatedAt 返回创建时间的副本。
func (b Banner) CreatedAt() time.Time { return b.createdAt }

// UpdatedAt 返回更新时间的副本。
func (b Banner) UpdatedAt() time.Time { return b.updatedAt }

// Cover 返回图片链接。
func (b Banner) Cover() string { return b.cover }

// Href 返回跳转链接。
func (b Banner) Href() string { return b.href }

// Show 返回是否显示。
func (b Banner) Show() bool { return b.show }

// Type 返回轮播图类型。
func (b Banner) Type() enum.BannerType { return b.bannerType }

// PartitionID 返回分区 ID。
func (b Banner) PartitionID() uint { return b.partitionID }

// BannerChanges 轮播图部分字段更新集合，nil 表示不更新。
//
// 仓储将该结构转换为 GORM 更新集，数据库列名只在持久化层使用。
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

// 后台分页默认值。
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

// resolveBannerPartitionID 依据轮播图类型裁定分区 ID。
//
// 规则：
//   - 非首页类型不允许携带分区 ID（携带即报 ErrBannerPartitionTypeUnsupported），一律归零；
//   - 首页类型的 0 表示首页本身，无需校验分区是否存在；
//   - 首页类型的非 0 分区需要调用方另行校验分区存在性，此时 needExists 为 true。
//
// 领域层只裁定规则，分区存在性属于 IO，由应用层经查询端口完成。
func (*Banner) resolveBannerPartitionID(bannerType enum.BannerType, partitionID uint) (resolved uint, needExists bool, err error) {
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

// ChangePlacement 统一维护类型与分区：切换为非首页类型时清除旧分区，
// 显式为非首页指定分区则拒绝。校验失败时实体保持不变。
func (b *Banner) ChangePlacement(bannerType *enum.BannerType, partitionID *uint) error {
	nextType, nextPartition := b.bannerType, b.partitionID
	if bannerType != nil && *bannerType > 0 {
		nextType = *bannerType
	}

	if partitionID != nil {
		nextPartition = *partitionID
	} else if nextType != enum.BannerTypeHome {
		nextPartition = 0
	}

	resolved, _, err := b.resolveBannerPartitionID(nextType, nextPartition)
	if err != nil {
		return err
	}

	b.bannerType, b.partitionID = nextType, resolved

	return nil
}

// NeedsPartitionCheck 告诉调用方是否需要从视频上下文取得分区存在性事实。
func (b *Banner) NeedsPartitionCheck() bool {
	return b.bannerType == enum.BannerTypeHome && b.partitionID > 0
}

// VisibleQuery 裁定前台可见列表的类型与分区。
//
// 规则：非首页类型携带分区筛选则拒绝；
// 只带分区未带类型时视为首页；首页未带分区时回落为 0。失败时不产生筛选结果。
func (b Banner) VisibleQuery(rawType int, partitionID *uint) (enum.BannerType, *uint, error) {
	bannerType, err := b.normalizeListedType(rawType, partitionID)
	if err != nil {
		return 0, nil, err
	}

	if bannerType != enum.BannerTypeHome {
		return bannerType, nil, nil
	}

	if partitionID != nil {
		return bannerType, partitionID, nil
	}

	homePartitionID := uint(0)

	return bannerType, &homePartitionID, nil
}

// AdminQueryType 裁定后台列表类型。
//
// 分区筛选同样只允许首页类型，但未指定的分区不会回落为 0。
func (b Banner) AdminQueryType(rawType int, partitionID *uint) (enum.BannerType, error) {
	return b.normalizeListedType(rawType, partitionID)
}

func (b Banner) normalizeListedType(rawType int, partitionID *uint) (enum.BannerType, error) {
	if err := b.rejectNonHomePartitionFilter(rawType, partitionID); err != nil {
		return 0, err
	}

	bannerType := b.bannerTypeOrZero(rawType)
	if partitionID != nil && bannerType == 0 {
		bannerType = enum.BannerTypeHome
	}

	return bannerType, nil
}

func (Banner) rejectNonHomePartitionFilter(rawType int, partitionID *uint) error {
	if partitionID != nil && *partitionID > 0 && rawType > 0 && rawType != int(enum.BannerTypeHome) {
		return ErrBannerPartitionFilterUnsupported
	}

	return nil
}

func (Banner) bannerTypeOrZero(raw int) enum.BannerType {
	if raw > 0 {
		return enum.BannerType(raw)
	}

	return 0
}
