package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// PartitionService 视频分区用例服务（原 service/video_service/partition_service.go）。
type PartitionService struct {
	partitions domain.PartitionRepository
	videos     domain.VideoRepository
	cache      PartitionCache
	authors    *AuthorService
}

// NewPartitionService 构造分区用例服务。
func NewPartitionService(
	partitions domain.PartitionRepository,
	videos domain.VideoRepository,
	cache PartitionCache,
	authors *AuthorService,
) *PartitionService {
	return &PartitionService{partitions: partitions, videos: videos, cache: cache, authors: authors}
}

// Create 创建分区。
func (s *PartitionService) Create(ctx context.Context, p *domain.Partition) error {
	if p == nil {
		return domain.ErrParamInvalid
	}

	if err := s.partitions.Create(ctx, p); err != nil {
		return err
	}

	s.cache.Invalidate()

	return nil
}

// Update 更新分区，并批量失效该分区下作品的详情 meta。
func (s *PartitionService) Update(ctx context.Context, p *domain.Partition) error {
	if p == nil || p.ID == 0 {
		return domain.ErrParamInvalid
	}

	if err := s.partitions.Update(ctx, p); err != nil {
		return err
	}

	s.cache.Invalidate()
	// 分区名/图标嵌入视频详情 meta，变更后批量失效该分区下作品 meta。
	s.authors.InvalidateVideoMetaCacheByPartition(ctx, p.ID)

	return nil
}

// Delete 删除分区（分区下有作品时拒绝）。
func (s *PartitionService) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return domain.ErrParamInvalid
	}

	count, err := s.videos.CountByPartition(ctx, id)
	if err != nil {
		return err
	}

	if count > 0 {
		return domain.ErrPartitionHasVideos
	}

	rows, err := s.partitions.Delete(ctx, id)
	if err != nil {
		return err
	}

	s.cache.Invalidate()
	zap.L().Info("delete partition ok", zap.Uint("partitionId", id), zap.Int64("rows", rows))

	return nil
}

// List 分页列出分区。
func (s *PartitionService) List(ctx context.Context, page, pageSize int) ([]PartitionItem, int64, error) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	partitions, total, err := s.partitions.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []PartitionItem{}, 0, nil
	}

	return toPartitionItems(partitions), total, nil
}

// ListSubmittable 返回允许投稿的分区列表（启用且允许投稿）。
func (s *PartitionService) ListSubmittable(ctx context.Context) ([]PartitionItem, error) {
	if cached, ok := s.cache.Load(); ok {
		return cached, nil
	}

	partitions, err := s.partitions.ListSubmittable(ctx)
	if err != nil {
		return nil, err
	}

	items := toPartitionItems(partitions)
	s.cache.Store(items)

	return items, nil
}

func toPartitionItems(partitions []domain.Partition) []PartitionItem {
	items := make([]PartitionItem, 0, len(partitions))
	for _, p := range partitions {
		items = append(items, PartitionItem{
			ID:            p.ID,
			Name:          p.Name,
			Icon:          p.Icon,
			SortOrder:     p.SortOrder,
			IsActive:      p.IsActive,
			IsSubmittable: p.IsSubmittable,
		})
	}

	return items
}
