package contentops

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ReinstateDefault 重新启用初始化横幅，同时维护类型与分区的关联规则。
func (b *Banner) ReinstateDefault(bannerType enum.BannerType) (BannerChanges, error) {
	if err := b.ChangePlacement(&bannerType, nil); err != nil {
		return BannerChanges{}, err
	}
	b.show = true
	typ, partition, show := b.bannerType, b.partitionID, b.show
	return BannerChanges{Type: &typ, PartitionID: &partition, Show: &show}, nil
}

type DefaultBannerRepository interface {
	FindByCoverForUpdate(ctx context.Context, cover string) (*Banner, error)
	Create(ctx context.Context, banner *Banner) (*Banner, error)
	Update(ctx context.Context, bannerID uint, changes BannerChanges) error
}
