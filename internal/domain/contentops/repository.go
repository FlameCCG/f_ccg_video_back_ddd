package contentops

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BannerRepository 轮播图聚合的仓储（接口所有权归领域层，实现在 infrastructure）。
type BannerRepository interface {
	// ListVisible 按类型与分区读取可展示的轮播图，id 倒序。
	// 无匹配数据时返回空切片（响应序列化为 []，与原实现一致）。
	ListVisible(ctx context.Context, bannerType enum.BannerType, partitionID *uint) ([]Banner, error)
	// ListAdmin 后台分页列表，返回当页数据与总数；总数为 0 时返回空切片。
	ListAdmin(ctx context.Context, query BannerAdminQuery) ([]Banner, int64, error)
	// ListVisibleByIDs 按给定 ID 顺序读取可展示的轮播图，缺失的 ID 直接跳过。
	ListVisibleByIDs(ctx context.Context, bannerType enum.BannerType, ids []uint) ([]Banner, error)
	// FindByID 按主键读取轮播图；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, bannerID uint) (*Banner, error)
	// Create 新增轮播图，并把自增主键与时间戳回填到入参实体。
	Create(ctx context.Context, banner *Banner) error
	// Update 按主键部分更新轮播图字段。
	Update(ctx context.Context, bannerID uint, changes BannerChanges) error
	// Delete 按主键批量删除轮播图。
	Delete(ctx context.Context, ids []uint) error
}
