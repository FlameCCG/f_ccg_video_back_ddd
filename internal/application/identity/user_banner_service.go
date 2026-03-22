package identity

import (
	"context"
	"path"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/uploadcheck"

	"go.uber.org/zap"
)

// siteConfigSection 站点配置在 YAML 中的段名。
const siteConfigSection = "site"

// UserBannerService 用户主页横幅用例服务。
type UserBannerService struct {
	cfgProvider  shared.ConfigStore
	confs        identitydomain.UserConfRepository
	banners      UserBannerCatalog
	configWriter ConfigFileWriter
	storage      BannerStorage
	logger       *zap.Logger
	bannerRules  identitydomain.BannerPolicy
}

// NewUserBannerService 构造用户横幅服务。
func NewUserBannerService(
	cfgProvider shared.ConfigStore,
	confs identitydomain.UserConfRepository,
	banners UserBannerCatalog,
	configWriter ConfigFileWriter,
	storage BannerStorage,
	logger *zap.Logger,
) *UserBannerService {
	return &UserBannerService{
		cfgProvider:  cfgProvider,
		confs:        confs,
		banners:      banners,
		configWriter: configWriter,
		storage:      storage,
		logger:       logger,
		bannerRules:  identitydomain.BannerPolicy{},
	}
}

// ListDefaults 返回系统默认用户主页横幅列表与当前用户的选择。
//
// viewerID 为 0（未登录）时当前选择返回零值。
func (s *UserBannerService) ListDefaults(ctx context.Context, viewerID uint) (*DefaultUserBannerList, error) {
	bannerIDs := s.systemDefaultBannerIDs()

	list, err := s.banners.ListVisible(ctx, bannerIDs)
	if err != nil {
		return nil, ErrBannerListFailed
	}

	currentBannerID, currentBannerImageURL := s.resolveCurrentSelection(ctx, viewerID)

	return &DefaultUserBannerList{
		BannerIDs:             append([]uint(nil), bannerIDs...),
		CurrentBannerID:       currentBannerID,
		CurrentBannerImageURL: currentBannerImageURL,
		List:                  list,
	}, nil
}

// resolveCurrentSelection 解析当前用户选中的横幅。
//
// 自定义横幅 URL 优先，此时 bannerID 返回 0。
func (s *UserBannerService) resolveCurrentSelection(ctx context.Context, viewerID uint) (uint, string) {
	if viewerID == 0 {
		return 0, ""
	}

	conf, err := s.confs.FindByUserID(ctx, viewerID)
	if err != nil || conf == nil {
		return 0, ""
	}

	bannerID, customBannerURL := conf.ListedBanner()
	if customBannerURL != "" || bannerID == 0 {
		return 0, customBannerURL
	}

	banner, err := s.banners.FindVisible(ctx, bannerID)
	if err != nil || banner == nil {
		return bannerID, ""
	}

	return bannerID, banner.Cover
}

// AdminSetDefault 更换注册默认横幅：写回配置并把旧默认横幅的用户批量迁移到新横幅。
func (s *UserBannerService) AdminSetDefault(ctx context.Context, bannerID uint) error {
	if err := s.ensureUserBannerSelectable(ctx, bannerID); err != nil {
		return identitydomain.ErrUserBannerNotFound
	}

	var oldDefaultBannerID uint
	if err := s.updateSiteConfig(func(cfgSite *site.Site) error {
		change, err := s.bannerRules.ReplaceRegisterDefault(cfgSite.DefaultUserBannerID, cfgSite.DefaultUserBannerIDs, bannerID)
		if err != nil {
			return err
		}
		oldDefaultBannerID = change.PreviousID
		cfgSite.DefaultUserBannerID = change.RegisterID
		cfgSite.DefaultUserBannerIDs = change.Defaults
		return nil
	}); err != nil {
		return err
	}

	if err := s.confs.ReassignBanner(ctx, oldDefaultBannerID, bannerID); err != nil {
		return ErrSetUserBannerFailed
	}

	return nil
}

// AdminUpdateDefaults 增删系统默认用户主页横幅列表。
func (s *UserBannerService) AdminUpdateDefaults(
	ctx context.Context,
	bannerIDs []uint,
	action int,
) (*AdminUpdateUserBannerListResult, error) {
	if action == identitydomain.UserBannerActionAdd {
		if err := s.ensureUserBannersSelectable(ctx, bannerIDs); err != nil {
			return nil, identitydomain.ErrUserBannerNotFound
		}
	}
	var current []uint
	if err := s.updateSiteConfig(func(cfgSite *site.Site) error {
		var err error
		current, err = s.bannerRules.EditDefaults(cfgSite.DefaultUserBannerIDs, cfgSite.DefaultUserBannerID, bannerIDs, action)
		if err != nil {
			return err
		}
		cfgSite.DefaultUserBannerIDs = append([]uint(nil), current...)
		return nil
	}); err != nil {
		return nil, err
	}

	return &AdminUpdateUserBannerListResult{BannerIDs: current}, nil
}

// UploadBanner 校验并保存用户上传的主页横幅，返回可访问 URL。
func (s *UserBannerService) UploadBanner(ctx context.Context, in UploadBannerInput) (string, error) {
	if err := s.storage.ValidateUploadID(in.FileHash); err != nil {
		return "", err
	}

	ext, contentType, err := uploadcheck.ValidateImage(in.File, in.Filename, in.Size, uploadcheck.DefaultMaxImageBytes)
	if err != nil {
		return "", err
	}

	if err = s.storage.EnsureReady(); err != nil {
		return "", ErrStorageConfigInvalid
	}

	relativePath := path.Join("banners", "user", in.FileHash+ext)

	bannerURL, err := s.storage.SaveBanner(ctx, relativePath, in.Size, in.File, contentType)
	if err != nil {
		s.logger.Error("save banner failed", zap.Error(err))

		return "", ErrBannerUploadFailed
	}

	return bannerURL, nil
}

// ensureUserBannerSelectable 校验单个横幅是可见的用户主页横幅。
func (s *UserBannerService) ensureUserBannerSelectable(ctx context.Context, bannerID uint) error {
	if err := s.bannerRules.RequireID(bannerID); err != nil {
		return err
	}

	banner, err := s.banners.FindVisible(ctx, bannerID)
	if err != nil {
		return err
	}

	return s.bannerRules.EnsureFound(banner != nil)
}

// ensureUserBannersSelectable 校验一批横幅全部是可见的用户主页横幅。
func (s *UserBannerService) ensureUserBannersSelectable(ctx context.Context, bannerIDs []uint) error {
	normalized := s.bannerRules.NormalizeIDs(bannerIDs)
	if err := s.bannerRules.RequireIDs(normalized); err != nil {
		return err
	}

	list, err := s.banners.ListVisible(ctx, normalized)
	if err != nil {
		return err
	}

	return s.bannerRules.EnsureAvailable(len(normalized), len(list))
}

// updateSiteConfig 与管理端及热更新共用配置锁，持久化失败时不发布。
func (s *UserBannerService) updateSiteConfig(fn func(*site.Site) error) error {
	return s.cfgProvider.Update(func(next *config.Config) error {
		var nextSite site.Site
		if next.Site != nil {
			nextSite = *next.Site
		}
		nextSite.DefaultUserBannerIDs = append([]uint(nil), nextSite.DefaultUserBannerIDs...)
		if err := fn(&nextSite); err != nil {
			return err
		}
		next.Site = &nextSite
		if err := s.configWriter.Write(siteConfigSection, next.Site); err != nil {
			return ErrPersistSiteConfigFailed
		}
		return nil
	})
}

// systemDefaultBannerIDs 汇总系统默认横幅集合（配置列表 + 注册默认横幅）。
func (s *UserBannerService) systemDefaultBannerIDs() []uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return []uint{}
	}

	return s.bannerRules.SystemDefaults(cfg.Site.DefaultUserBannerIDs, cfg.Site.DefaultUserBannerID)
}
