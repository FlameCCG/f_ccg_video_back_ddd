package contentops

import (
	"context"
	"fmt"

	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// ErrBannerNotFound 轮播图不存在（转发领域错误，供 interfaces 层做错误分支判断，
// 避免 interfaces 层直接 import 领域包）。
var ErrBannerNotFound = contentopsdomain.ErrBannerNotFound

// PartitionResolveError 标记分区裁定失败，使接口层可区分裁定错误与落库错误。
//
// 分区裁定失败返回错误详情；落库失败返回固定文案「创建失败」。
type PartitionResolveError struct {
	// Err 裁定失败的原始错误。
	Err error
}

// Error 返回原始错误文案。
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

// BannerService 轮播图用例服务。
type BannerService struct {
	txm        tx.Manager
	banners    contentopsdomain.BannerRepository
	cache      BannerCache
	partitions VideoPartitionQuery
}

// NewBannerService 构造轮播图用例服务。
func NewBannerService(
	txm tx.Manager,
	banners contentopsdomain.BannerRepository,
	cache BannerCache,
	partitions VideoPartitionQuery,
) *BannerService {
	return &BannerService{txm: txm, banners: banners, cache: cache, partitions: partitions}
}

// ListVisible 前台可见轮播图列表（带 Redis 缓存）。
//
// 过滤条件裁定规则：
//   - 指定了非首页类型又带分区筛选 → ErrBannerPartitionFilterUnsupported；
//   - 只带分区未带类型 → 视为首页类型；
//   - 首页类型未带分区 → 分区回落 0（首页本身）。
func (s *BannerService) ListVisible(ctx context.Context, params VisibleBannerParams) ([]contentopsdomain.Banner, error) {
	bannerType, partitionID, err := (contentopsdomain.Banner{}).VisibleQuery(params.Type, params.PartitionID)
	if err != nil {
		return nil, err
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
// 分区筛选同样只允许首页类型，
// 但后台不会把未指定的分区回落为 0（不限分区）。
func (s *BannerService) ListAdmin(
	ctx context.Context,
	params AdminBannerListParams,
) ([]contentopsdomain.Banner, int64, error) {
	bannerType, err := (contentopsdomain.Banner{}).AdminQueryType(params.Type, params.PartitionID)
	if err != nil {
		return nil, 0, err
	}

	query := contentopsdomain.BannerAdminQuery{
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
) ([]contentopsdomain.Banner, error) {
	if len(ids) == 0 {
		return []contentopsdomain.Banner{}, nil
	}

	return s.banners.ListVisibleByIDs(ctx, bannerType, ids)
}

// Create 创建轮播图，返回落库后的实体（含自增主键）。
//
// 默认值：未指定显示状态默认显示，
// 未指定类型默认首页轮播图。
func (s *BannerService) Create(ctx context.Context, params CreateBannerParams) (*contentopsdomain.Banner, error) {
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

	banner := contentopsdomain.NewBanner(params.Cover, params.Href, show)
	if err := banner.ChangePlacement(&bannerType, &partitionID); err != nil {
		return nil, &PartitionResolveError{Err: err}
	}

	if err := s.ensureBannerPartition(ctx, banner); err != nil {
		return nil, err
	}

	saved, err := s.banners.Create(ctx, banner)
	if err != nil {
		return nil, err
	}

	s.invalidateCache()

	return saved, nil
}

// Update 部分更新轮播图。
//
// 类型与分区的关联修改在同一事务内锁定最新状态，再裁定并按字段保存。
// 仅改封面等无关联字段时沿用直接更新；缓存只在提交成功后失效。
func (s *BannerService) Update(ctx context.Context, params UpdateBannerParams) error {
	changes := contentopsdomain.BannerChanges{
		Cover: params.Cover,
		Href:  params.Href,
		Show:  params.Show,
	}

	var err error
	if params.Type != nil || params.PartitionID != nil {
		if params.ID == 0 {
			return fmt.Errorf("%w: %w", contentopsdomain.ErrBannerNotFound, contentopsdomain.ErrBannerInvalidParam)
		}
		started := false
		err = s.txm.Do(ctx, func(txCtx context.Context) error {
			started = true
			return s.updatePlacement(txCtx, params, changes)
		})
		if err != nil && !started {
			// 开始事务失败时按未能读取记录处理，返回查询失败分类。
			return fmt.Errorf("%w: %w", contentopsdomain.ErrBannerNotFound, err)
		}
	} else if err = changes.ValidateFor(params.ID); err == nil {
		err = s.banners.Update(ctx, params.ID, changes)
	}
	if err != nil {
		return err
	}

	s.invalidateCache()

	return nil
}

func (s *BannerService) updatePlacement(ctx context.Context, params UpdateBannerParams, changes contentopsdomain.BannerChanges) error {
	current, err := s.banners.FindByIDForUpdate(ctx, params.ID)
	if err != nil {
		return fmt.Errorf("%w: %w", contentopsdomain.ErrBannerNotFound, err)
	}
	if current == nil {
		return contentopsdomain.ErrBannerNotFound
	}
	var requestedType *enum.BannerType
	if params.Type != nil {
		value := enum.BannerType(*params.Type)
		requestedType = &value
	}
	if err := current.ChangePlacement(requestedType, params.PartitionID); err != nil {
		return &PartitionResolveError{Err: err}
	}
	if err := s.ensureBannerPartition(ctx, current); err != nil {
		return err
	}
	if requestedType != nil && *requestedType > 0 {
		nextType := current.Type()
		changes.Type = &nextType
	}
	nextPartition := current.PartitionID()
	changes.PartitionID = &nextPartition
	if err := changes.ValidateFor(params.ID); err != nil {
		return err
	}
	return s.banners.Update(ctx, params.ID, changes)
}

// Delete 批量删除轮播图；ID 列表为空时直接返回成功（不清缓存）。
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

// Get 按主键读取轮播图。
//
// 缺少主键、记录不存在或读取失败均返回 ErrBannerNotFound。
func (s *BannerService) Get(ctx context.Context, bannerID uint) (*contentopsdomain.Banner, error) {
	if bannerID == 0 {
		return nil, fmt.Errorf("%w: %w", contentopsdomain.ErrBannerNotFound, contentopsdomain.ErrBannerInvalidParam)
	}

	banner, err := s.banners.FindByID(ctx, bannerID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", contentopsdomain.ErrBannerNotFound, err)
	}

	if banner == nil {
		return nil, contentopsdomain.ErrBannerNotFound
	}

	return banner, nil
}

// ensureBannerPartition 只取得分区存在性事实；类型与分区组合由 Banner 保护。
func (s *BannerService) ensureBannerPartition(ctx context.Context, banner *contentopsdomain.Banner) error {
	if !banner.NeedsPartitionCheck() {
		return nil
	}
	exists, err := s.partitions.Exists(ctx, banner.PartitionID())
	if err != nil {
		return &PartitionResolveError{Err: err}
	}
	if !exists {
		return &PartitionResolveError{Err: contentopsdomain.ErrBannerPartitionNotFound}
	}
	return nil
}

// invalidateCache 清空可见轮播图缓存（写操作成功后调用）。
func (s *BannerService) invalidateCache() {
	if s.cache != nil {
		s.cache.InvalidateVisible()
	}
}
