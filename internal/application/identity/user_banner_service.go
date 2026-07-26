package identity

import (
	"context"
	"path"
	"slices"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/config/site"
	"f_ccg_video_back_ddd/pkg/uploadcheck"

	"go.uber.org/zap"
)

// siteConfigSection 站点配置在 YAML 中的段名。
const siteConfigSection = "site"

// userBannerObjectPrefix 用户横幅在存储中的相对目录（与原实现一致）。
var userBannerObjectPrefix = []string{"banners", "user"}

// UserBannerService 用户主页横幅用例服务
// （原 api/user_api/user_default_banner.go、user_banner_setting.go、upload_banner.go）。
type UserBannerService struct {
	cfgProvider  *config.Provider
	confs        domain.UserConfRepository
	banners      UserBannerCatalog
	configWriter ConfigFileWriter
	storage      BannerStorage
}

// NewUserBannerService 构造用户横幅服务。
func NewUserBannerService(
	cfgProvider *config.Provider,
	confs domain.UserConfRepository,
	banners UserBannerCatalog,
	configWriter ConfigFileWriter,
	storage BannerStorage,
) *UserBannerService {
	return &UserBannerService{
		cfgProvider:  cfgProvider,
		confs:        confs,
		banners:      banners,
		configWriter: configWriter,
		storage:      storage,
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
// 自定义横幅 URL 优先，此时 bannerID 返回 0（等价原 resolveCurrentUserBannerSelection）。
func (s *UserBannerService) resolveCurrentSelection(ctx context.Context, viewerID uint) (uint, string) {
	if viewerID == 0 {
		return 0, ""
	}

	conf, err := s.confs.FindByUserID(ctx, viewerID)
	if err != nil || conf == nil {
		return 0, ""
	}

	if customBannerURL := strings.TrimSpace(conf.BannerURL); customBannerURL != "" {
		return 0, customBannerURL
	}

	if conf.BannerID == 0 {
		return 0, ""
	}

	banner, err := s.banners.FindVisible(ctx, conf.BannerID)
	if err != nil || banner == nil {
		return conf.BannerID, ""
	}

	return conf.BannerID, banner.Cover
}

// AdminSetDefault 更换注册默认横幅：写回配置并把旧默认横幅的用户批量迁移到新横幅。
func (s *UserBannerService) AdminSetDefault(ctx context.Context, bannerID uint) error {
	if err := s.ensureUserBannerSelectable(ctx, bannerID); err != nil {
		return domain.ErrUserBannerNotFound
	}

	s.ensureSiteConfig()

	oldDefaultBannerID := domain.ResolveDefaultUserBannerID(s.configuredDefaultBannerID())
	if bannerID == oldDefaultBannerID {
		return domain.ErrBannerAlreadyRegisterDefault
	}

	s.replaceSiteConfig(func(cfgSite *site.Site) {
		cfgSite.DefaultUserBannerID = bannerID

		if oldDefaultBannerID > 0 && oldDefaultBannerID != bannerID {
			if !slices.Contains(cfgSite.DefaultUserBannerIDs, oldDefaultBannerID) {
				cfgSite.DefaultUserBannerIDs = append(cfgSite.DefaultUserBannerIDs, oldDefaultBannerID)
			}
		}
	})

	if err := s.persistSiteConfig(); err != nil {
		return ErrPersistSiteConfigFailed
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
	s.ensureSiteConfig()

	// 注意：此处按原实现读取「未经兜底」的配置值，与 AdminSetDefault 的语义不同。
	if err := domain.EnsureRegisterDefaultBannerNotTouched(bannerIDs, s.configuredDefaultBannerID()); err != nil {
		return nil, domain.ErrRegisterDefaultBannerLocked
	}

	current := s.editableBannerIDs()

	switch action {
	case domain.UserBannerActionAdd:
		if err := s.ensureUserBannersSelectable(ctx, bannerIDs); err != nil {
			return nil, domain.ErrUserBannerNotFound
		}
		// 校验：如果要添加的 bannerID 已经存在于系统默认列表中，则报错。
		for _, id := range domain.NormalizeBannerIDs(bannerIDs) {
			if slices.Contains(current, id) {
				return nil, domain.ErrBannerAlreadyInDefaults
			}
		}

		current = domain.MergeBannerIDs(current, bannerIDs)
	case domain.UserBannerActionDelete:
		current = domain.RemoveBannerIDs(current, bannerIDs)
	default:
		return nil, domain.ErrBannerActionInvalid
	}

	s.replaceSiteConfig(func(cfgSite *site.Site) {
		cfgSite.DefaultUserBannerIDs = append([]uint(nil), current...)
	})

	if err := s.persistSiteConfig(); err != nil {
		return nil, ErrPersistSiteConfigFailed
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

	relativePath := path.Join(append(append([]string{}, userBannerObjectPrefix...), in.FileHash+ext)...)

	bannerURL, err := s.storage.SaveBanner(ctx, relativePath, in.Size, in.File, contentType)
	if err != nil {
		zap.L().Error("save banner failed", zap.Error(err))

		return "", ErrBannerUploadFailed
	}

	return bannerURL, nil
}

// ensureUserBannerSelectable 校验单个横幅是可见的用户主页横幅。
func (s *UserBannerService) ensureUserBannerSelectable(ctx context.Context, bannerID uint) error {
	if bannerID == 0 {
		return domain.ErrBannerIDRequired
	}

	banner, err := s.banners.FindVisible(ctx, bannerID)
	if err != nil {
		return err
	}

	if banner == nil {
		return domain.ErrUserBannerNotFound
	}

	return nil
}

// ensureUserBannersSelectable 校验一批横幅全部是可见的用户主页横幅。
func (s *UserBannerService) ensureUserBannersSelectable(ctx context.Context, bannerIDs []uint) error {
	normalized := domain.NormalizeBannerIDs(bannerIDs)
	if len(normalized) == 0 {
		return domain.ErrBannerIDsRequired
	}

	list, err := s.banners.ListVisible(ctx, normalized)
	if err != nil {
		return err
	}

	if len(list) != len(normalized) {
		return domain.ErrUserBannerNotFound
	}

	return nil
}

// ensureSiteConfig 保证配置树中存在 Site 段（浅拷贝后原子发布）。
func (s *UserBannerService) ensureSiteConfig() {
	cfg := s.cfgProvider.Get()
	if cfg != nil && cfg.Site != nil {
		return
	}

	next := config.Clone(cfg)
	if next.Site == nil {
		next.Site = &site.Site{}
	}

	s.cfgProvider.Set(next)
}

// replaceSiteConfig 浅拷贝 Site 后修改并原子发布，避免原地改写共享配置树。
func (s *UserBannerService) replaceSiteConfig(fn func(cfgSite *site.Site)) {
	s.ensureSiteConfig()

	cfg := config.Clone(s.cfgProvider.Get())

	var nextSite site.Site
	if cfg.Site != nil {
		nextSite = *cfg.Site
	}

	fn(&nextSite)
	cfg.Site = &nextSite
	s.cfgProvider.Set(cfg)
}

// persistSiteConfig 把当前 Site 配置段写回配置文件。
func (s *UserBannerService) persistSiteConfig() error {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return ErrSiteConfigNotInitialized
	}

	return s.configWriter.Write(siteConfigSection, cfg.Site)
}

// systemDefaultBannerIDs 汇总系统默认横幅集合（配置列表 + 注册默认横幅）。
func (s *UserBannerService) systemDefaultBannerIDs() []uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return []uint{}
	}

	return domain.SystemDefaultUserBannerIDs(cfg.Site.DefaultUserBannerIDs, cfg.Site.DefaultUserBannerID)
}

// editableBannerIDs 返回可编辑的系统默认横幅列表（不含注册默认横幅）。
func (s *UserBannerService) editableBannerIDs() []uint {
	s.ensureSiteConfig()

	cfg := s.cfgProvider.Get()
	if cfg != nil && cfg.Site != nil && cfg.Site.DefaultUserBannerIDs != nil {
		return domain.NormalizeBannerIDs(cfg.Site.DefaultUserBannerIDs)
	}

	return []uint{}
}

func (s *UserBannerService) configuredDefaultBannerID() uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return 0
	}

	return cfg.Site.DefaultUserBannerID
}
