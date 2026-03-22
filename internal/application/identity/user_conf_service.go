package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/application/shared"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
)

// UserConfService 用户配置用例服务。
type UserConfService struct {
	cfgProvider shared.ConfigReader
	confs       identitydomain.UserConfRepository
	query       UserConfQuery
	banners     UserBannerCatalog
	bannerRules identitydomain.BannerPolicy
}

// NewUserConfService 构造用户配置服务。
func NewUserConfService(
	cfgProvider shared.ConfigReader,
	confs identitydomain.UserConfRepository,
	query UserConfQuery,
	banners UserBannerCatalog,
) *UserConfService {
	return &UserConfService{
		cfgProvider: cfgProvider,
		confs:       confs,
		query:       query,
		banners:     banners,
		bannerRules: identitydomain.BannerPolicy{},
	}
}

// View 读取用户配置读模型（likeTags 保持数据库原始 JSON）。
func (s *UserConfService) View(ctx context.Context, userID uint) (*UserConfView, error) {
	view, err := s.query.View(ctx, userID)
	if err != nil || view == nil {
		return nil, identitydomain.ErrUserConfNotFound
	}

	return view, nil
}

// Update 部分更新用户配置。
//
// 选择系统横幅时会清空自定义横幅 URL；两者同时传入时以 bannerId 为准。
func (s *UserConfService) Update(ctx context.Context, in UpdateUserConfInput) error {
	conf, err := s.confs.FindByUserID(ctx, in.UserID)
	if err != nil || conf == nil {
		return identitydomain.ErrUserConfNotFound
	}

	if in.BannerID != nil {
		if err = s.ensureSystemDefaultBannerSelectable(ctx, *in.BannerID); err != nil {
			return identitydomain.ErrSystemDefaultBannerNotFound
		}
	}

	changes, err := conf.Plan(identitydomain.UserConfUpdate{
		OpenCollect:   in.OpenCollect,
		OpenFans:      in.OpenFans,
		OpenFollow:    in.OpenFollow,
		OpenLikeVideo: in.OpenLikeVideo,
		OpenCoinVideo: in.OpenCoinVideo,
		BannerID:      in.BannerID,
		BannerURL:     in.BannerURL,
		LikeTags:      in.LikeTags,
	})
	if err != nil {
		return err
	}

	if err = s.confs.Update(ctx, in.UserID, changes); err != nil {
		return ErrUpdateUserFailed
	}

	return nil
}

// ensureSystemDefaultBannerSelectable 校验横幅位于系统默认列表中且可见。
func (s *UserConfService) ensureSystemDefaultBannerSelectable(ctx context.Context, bannerID uint) error {
	if err := s.bannerRules.RequireID(bannerID); err != nil {
		return err
	}
	configuredBannerIDs := s.systemDefaultBannerIDs()
	if err := s.bannerRules.EnsureSystemDefaultID(bannerID, configuredBannerIDs); err != nil {
		return err
	}

	list, err := s.banners.ListVisible(ctx, []uint{bannerID})
	if err != nil {
		return err
	}

	return s.bannerRules.EnsureSystemDefaultVisible(len(list))
}

// systemDefaultBannerIDs 汇总系统默认横幅集合（配置列表 + 注册默认横幅）。
func (s *UserConfService) systemDefaultBannerIDs() []uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return []uint{}
	}

	return s.bannerRules.SystemDefaults(cfg.Site.DefaultUserBannerIDs, cfg.Site.DefaultUserBannerID)
}
