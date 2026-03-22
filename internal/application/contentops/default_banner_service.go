package contentops

import (
	"context"
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// DefaultBannerService 在同一事务中重读、裁定并保存初始化横幅。
type DefaultBannerService struct {
	txm     tx.Manager
	banners contentopsdomain.DefaultBannerRepository
}

func NewDefaultBannerService(txm tx.Manager, banners contentopsdomain.DefaultBannerRepository) *DefaultBannerService {
	return &DefaultBannerService{txm: txm, banners: banners}
}

func (s *DefaultBannerService) UpsertBannerByCover(ctx context.Context, cover string, bannerType enum.BannerType) (bool, error) {
	created := false
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		banner, err := s.banners.FindByCoverForUpdate(txCtx, cover)
		if err != nil || banner == nil {
			// 查询失败时按记录不存在处理，并尝试创建。
			banner = contentopsdomain.NewBanner(cover, "", true)
			if _, err := banner.ReinstateDefault(bannerType); err != nil {
				return err
			}
			if _, err := s.banners.Create(txCtx, banner); err != nil {
				return err
			}
			created = true
			return nil
		}
		change, err := banner.ReinstateDefault(bannerType)
		if err != nil {
			return err
		}
		return s.banners.Update(txCtx, banner.ID(), change)
	})
	return created && err == nil, err
}
