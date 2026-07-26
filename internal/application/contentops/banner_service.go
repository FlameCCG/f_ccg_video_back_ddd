package contentops

import (
	"context"
	"fmt"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ErrBannerNotFound 轮播图不存在（转发领域错误，供 interfaces 层做错误分支判断，
// 避免 interfaces 层直接 import 领域包）。
var ErrBannerNotFound = domain.ErrBannerNotFound

// PartitionResolveError 标记「分区裁定阶段」的失败。
//
// 原 api/banner_api 在分区裁定失败时回显 err.Error()，而在落库失败时回显固定文案
// （"创建失败"）。新架构把裁定与落库收敛进同一个用例方法，
// 用本类型让 interfaces 层区分两个阶段，保持响应文案与原实现逐字一致。
type PartitionResolveError struct {
	// Err 裁定失败的原始错误。
	Err error
}

// Error 返回原始错误文案（与原实现回显的内容一致）。
func (e *PartitionResolveError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

// Unwrap 返回原始错误，支持 errors.Is / errors.As 穿透。
func (e *PartitionResolveError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// BannerService 轮播图用例服务（原 service/banner_service + api/banner_api 中的裁定逻辑）。
type BannerService struct {
	banners    domain.BannerRepository
	cache      BannerCache
	partitions VideoPartitionQuery
}

// NewBannerService 构造轮播图用例服务。
func NewBannerService(
	banners domain.BannerRepository,
	cache BannerCache,
	partitions VideoPartitionQuery,
) *BannerService {
	return &BannerService{banners: banners, cache: cache, partitions: partitions}
}

// ListVisible 前台可见轮播图列表（带 Redis 缓存）。
//
// 过滤条件裁定规则与原 api/banner_api.ListView 逐行一致：
//   - 指定了非首页类型又带分区筛选 → ErrBannerPartitionFilterUnsupported；
//   - 只带分区未带类型 → 视为首页类型；
//   - 首页类型未带分区 → 分区回落 0（首页本身）。
func (s *BannerService) ListVisible(ctx context.Context, params VisibleBannerParams) ([]domain.Banner, error) {
	if params.PartitionID != nil && *params.PartitionID > 0 &&
		params.Type > 0 && params.Type != int(enum.BannerTypeHome) {
		return nil, ErrBannerPartitionFilterUnsupported
	}

	bannerType := bannerTypeOrZero(params.Type)

	var partitionID *uint

	if params.PartitionID != nil && bannerType == 0 {
		bannerType = enum.BannerTypeHome
	}

	if bannerType == enum.BannerTypeHome {
		if params.PartitionID != nil {
			partitionID = params.PartitionID
		} else {
			homePartitionID := uint(0)
			partitionID = &homePartitionID
		}
	}

	if s.cache != nil {
		if cached, ok := s.cache.LoadVisible(bannerType, partitionID); ok {
			return cached, nil
		}
	}

	list, err := s.banners.ListVisible(ctx, bannerType, partitionID)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		s.cache.StoreVisible(bannerType, partitionID, list)
	}

	return list, nil
}

// ListAdmin 后台轮播图分页列表。
//
// 与原 api/banner_api.AdminListView 一致：分区筛选同样只允许首页类型，
// 但后台不会把未指定的分区回落为 0（不限分区）。
func (s *BannerService) ListAdmin(
	ctx context.Context,
	params AdminBannerListParams,
) ([]domain.Banner, int64, error) {
	if params.PartitionID != nil && *params.PartitionID > 0 &&
		params.Type > 0 && params.Type != int(enum.BannerTypeHome) {
		return nil, 0, ErrBannerPartitionFilterUnsupported
	}

	bannerType := bannerTypeOrZero(params.Type)
	if params.PartitionID != nil && bannerType == 0 {
		bannerType = enum.BannerTypeHome
	}

	query := domain.BannerAdminQuery{
		Page:        params.Page,
		PageSize:    params.PageSize,
		Type:        bannerType,
		PartitionID: params.PartitionID,
		Show:        params.Show,
	}
	query.Normalize()

	return s.banners.ListAdmin(ctx, query)
}

// ListVisibleByIDs 按给定 ID 顺序读取可展示的轮播图（供用户默认横幅等场景复用）。
func (s *BannerService) ListVisibleByIDs(
	ctx context.Context,
	bannerType enum.BannerType,
	ids []uint,
) ([]domain.Banner, error) {
	if len(ids) == 0 {
		return []domain.Banner{}, nil
	}

	return s.banners.ListVisibleByIDs(ctx, bannerType, ids)
}

// Create 创建轮播图，返回落库后的实体（含自增主键）。
//
// 默认值与原 api/banner_api.AdminCreateView 一致：未指定显示状态默认显示，
// 未指定类型默认首页轮播图。
func (s *BannerService) Create(ctx context.Context, params CreateBannerParams) (*domain.Banner, error) {
	show := true
	if params.Show != nil {
		show = *params.Show
	}

	bannerType := enum.BannerTypeHome
	if params.Type > 0 {
		bannerType = enum.BannerType(params.Type)
	}

	var partitionID uint
	if params.PartitionID != nil {
		partitionID = *params.PartitionID
	}

	partitionID, err := s.resolvePartitionID(ctx, bannerType, partitionID)
	if err != nil {
		return nil, err
	}

	banner := &domain.Banner{
		Cover:       params.Cover,
		Href:        params.Href,
		Show:        show,
		Type:        bannerType,
		PartitionID: partitionID,
	}

	if err := s.banners.Create(ctx, banner); err != nil {
		return nil, err
	}

	s.invalidateCache()

	return banner, nil
}

// Update 部分更新轮播图。
//
// 与原 api/banner_api.AdminUpdateView 逐行一致：
// 仅当请求携带类型或分区时才读取当前记录并重新裁定分区，
// 且此时必定重写 partition_id 列。
func (s *BannerService) Update(ctx context.Context, params UpdateBannerParams) error {
	changes := domain.BannerChanges{
		Cover: params.Cover,
		Href:  params.Href,
		Show:  params.Show,
	}

	if params.Type != nil || params.PartitionID != nil {
		current, err := s.Get(ctx, params.ID)
		if err != nil {
			return err
		}

		bannerType := current.Type
		if params.Type != nil && *params.Type > 0 {
			bannerType = enum.BannerType(*params.Type)
			changes.Type = &bannerType
		}

		partitionID := current.PartitionID

		switch {
		case params.PartitionID != nil:
			partitionID = *params.PartitionID
		case bannerType != enum.BannerTypeHome:
			partitionID = 0
		}

		resolved, err := s.resolvePartitionID(ctx, bannerType, partitionID)
		if err != nil {
			return err
		}

		changes.PartitionID = &resolved
	}

	if params.ID == 0 {
		return domain.ErrBannerInvalidParam
	}

	if changes.IsEmpty() {
		return domain.ErrBannerNoUpdatableField
	}

	if err := s.banners.Update(ctx, params.ID, changes); err != nil {
		return err
	}

	s.invalidateCache()

	return nil
}

// Delete 批量删除轮播图；ID 列表为空时直接返回成功（与原实现一致，不清缓存）。
func (s *BannerService) Delete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	if err := s.banners.Delete(ctx, ids); err != nil {
		return err
	}

	s.invalidateCache()

	return nil
}

// Get 按主键读取轮播图（原 banner_service.BannerService.Get）。
//
// 任何失败（缺少主键、记录不存在、读取报错）都归为 ErrBannerNotFound，
// 与原 api 层「Get 出错即回显轮播图不存在」的处理一致。
func (s *BannerService) Get(ctx context.Context, bannerID uint) (*domain.Banner, error) {
	if bannerID == 0 {
		return nil, fmt.Errorf("%w: %w", domain.ErrBannerNotFound, domain.ErrBannerInvalidParam)
	}

	banner, err := s.banners.FindByID(ctx, bannerID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domain.ErrBannerNotFound, err)
	}

	if banner == nil {
		return nil, domain.ErrBannerNotFound
	}

	return banner, nil
}

// resolvePartitionID 裁定并校验分区 ID：领域规则判类型，查询端口判存在性。
func (s *BannerService) resolvePartitionID(
	ctx context.Context,
	bannerType enum.BannerType,
	partitionID uint,
) (uint, error) {
	resolved, needExists, err := domain.ResolveBannerPartitionID(bannerType, partitionID)
	if err != nil {
		return 0, &PartitionResolveError{Err: err}
	}

	if !needExists {
		return resolved, nil
	}

	exists, err := s.partitions.Exists(ctx, resolved)
	if err != nil {
		return 0, &PartitionResolveError{Err: err}
	}

	if !exists {
		return 0, &PartitionResolveError{Err: domain.ErrBannerPartitionNotFound}
	}

	return resolved, nil
}

// invalidateCache 清空可见轮播图缓存（写操作成功后调用）。
func (s *BannerService) invalidateCache() {
	if s.cache == nil {
		return
	}

	s.cache.InvalidateVisible()
}
