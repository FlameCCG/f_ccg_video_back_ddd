package identity

import (
	"context"
	"slices"
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

// UserConfService 用户配置用例服务（原 api/user_api/user_conf.go）。
type UserConfService struct {
	cfgProvider shared.ConfigReader
	confs       domain.UserConfRepository
	query       UserConfQuery
	banners     UserBannerCatalog
}

// NewUserConfService 构造用户配置服务。
func NewUserConfService(
	cfgProvider shared.ConfigReader,
	confs domain.UserConfRepository,
	query UserConfQuery,
	banners UserBannerCatalog,
) *UserConfService {
	return &UserConfService{cfgProvider: cfgProvider, confs: confs, query: query, banners: banners}
}

// View 读取用户配置读模型（likeTags 保持数据库原始 JSON）。
func (s *UserConfService) View(ctx context.Context, userID uint) (*UserConfView, error) {
	view, err := s.query.View(ctx, userID)
	if err != nil || view == nil {
		return nil, domain.ErrUserConfNotFound
	}

	return view, nil
}

// Update 部分更新用户配置。
//
// 选择系统横幅时会清空自定义横幅 URL；两者同时传入时以 bannerId 为准（与原实现一致）。
func (s *UserConfService) Update(ctx context.Context, in UpdateUserConfInput) error {
	conf, err := s.confs.FindByUserID(ctx, in.UserID)
	if err != nil || conf == nil {
		return domain.ErrUserConfNotFound
	}

	changes := domain.UserConfChanges{
		OpenCollect:   in.OpenCollect,
		OpenFans:      in.OpenFans,
		OpenFollow:    in.OpenFollow,
		OpenLikeVideo: in.OpenLikeVideo,
		OpenCoinVideo: in.OpenCoinVideo,
	}

	if in.LikeTags != nil {
		normalized := domain.NormalizeLikeTags(*in.LikeTags)
		changes.LikeTags = &normalized
	}

	bannerURL := strings.TrimSpace(in.BannerURL)

	switch {
	case in.BannerID != nil:
		if err = s.ensureSystemDefaultBannerSelectable(ctx, *in.BannerID); err != nil {
			return domain.ErrSystemDefaultBannerNotFound
		}

		bannerID := *in.BannerID
		cleared := ""
		changes.BannerID = &bannerID
		changes.BannerURL = &cleared
	case bannerURL != "":
		changes.BannerURL = &bannerURL
	}

	if changes.IsEmpty() {
		return domain.ErrNoFieldsToUpdate
	}

	if err = s.confs.Update(ctx, in.UserID, changes); err != nil {
		return ErrUpdateUserFailed
	}

	return nil
}

// ensureSystemDefaultBannerSelectable 校验横幅位于系统默认列表中且可见。
//
// 等价原 api/user_api/user_default_banner.go 的 ensureSystemDefaultUserBannerSelectable。
func (s *UserConfService) ensureSystemDefaultBannerSelectable(ctx context.Context, bannerID uint) error {
	if bannerID == 0 {
		return domain.ErrBannerIDRequired
	}

	configuredBannerIDs := s.systemDefaultBannerIDs()
	if len(configuredBannerIDs) == 0 {
		return domain.ErrSystemDefaultBannerNotFound
	}

	if !slices.Contains(configuredBannerIDs, bannerID) {
		return domain.ErrSystemDefaultBannerNotFound
	}

	list, err := s.banners.ListVisible(ctx, []uint{bannerID})
	if err != nil {
		return err
	}

	if len(list) != 1 {
		return domain.ErrSystemDefaultBannerNotFound
	}

	return nil
}

// systemDefaultBannerIDs 汇总系统默认横幅集合（配置列表 + 注册默认横幅）。
func (s *UserConfService) systemDefaultBannerIDs() []uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return []uint{}
	}

	return domain.SystemDefaultUserBannerIDs(cfg.Site.DefaultUserBannerIDs, cfg.Site.DefaultUserBannerID)
}
