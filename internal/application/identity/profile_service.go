package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/apperr"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"golang.org/x/sync/errgroup"
)

// ProfileService 用户资料读写用例服务
// （原 api/user_api/user_info.go、user_detail.go、admin_info.go、
// update_user_info.go、admin_update_user_info.go）。
type ProfileService struct {
	cfgProvider shared.ConfigReader
	txm         tx.Manager
	users       domain.UserAccountRepository
	confs       domain.UserConfRepository
	banners     UserBannerCatalog
	relations   UserRelationQuery
	roles       UserRoleQuery
	videoStats  PublishedVideoStatsQuery
	renamer     VideoAuthorRenamer
	caches      VideoCacheInvalidator
}

// NewProfileService 构造用户资料服务。
func NewProfileService(
	cfgProvider shared.ConfigReader,
	txm tx.Manager,
	users domain.UserAccountRepository,
	confs domain.UserConfRepository,
	banners UserBannerCatalog,
	relations UserRelationQuery,
	roles UserRoleQuery,
	videoStats PublishedVideoStatsQuery,
	renamer VideoAuthorRenamer,
	caches VideoCacheInvalidator,
) *ProfileService {
	return &ProfileService{
		cfgProvider: cfgProvider,
		txm:         txm,
		users:       users,
		confs:       confs,
		banners:     banners,
		relations:   relations,
		roles:       roles,
		videoStats:  videoStats,
		renamer:     renamer,
		caches:      caches,
	}
}

// UserInfo 当前登录用户信息（含关注/粉丝/动态计数与横幅）。
func (s *ProfileService) UserInfo(ctx context.Context, userID uint) (*UserInfoResult, error) {
	user, conf, err := s.users.FindWithConfByID(ctx, userID)
	if err != nil || user == nil {
		return nil, apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	var (
		counts    UserRelationCounts
		bannerURL string
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		counts = s.relations.RelationCounts(groupCtx, userID, true)

		return nil
	})
	group.Go(func() error {
		bannerURL = s.ResolveUserBannerURL(groupCtx, conf)

		return nil
	})
	_ = group.Wait()

	isAdmin, err := IsAdminUser(ctx, s.roles, userID)
	if err != nil {
		return nil, err
	}

	return &UserInfoResult{
		IsAdmin:        isAdmin,
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		Avatar:         user.Avatar,
		BannerURL:      bannerURL,
		Description:    user.Description,
		Gender:         user.Gender,
		Birthday:       domain.FormatBirthday(user.Birthday),
		Level:          user.Level,
		Exp:            user.Exp,
		CoinCount:      user.CoinCount,
		FollowCount:    counts.FocusCount,
		FansCount:      counts.FansCount,
		DynamicCount:   counts.DynamicCount,
		RegisterSource: user.RegisterSource,
		LikeTags:       resolveLikeTags(conf),
		HasPassword:    user.HasPassword(),
		IsThirdParty:   user.IsThirdParty(),
	}, nil
}

// UserDetail 指定用户的公开详情（含作品曝光汇总）。
func (s *ProfileService) UserDetail(ctx context.Context, userID uint) (*UserDetailResult, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	conf, err := s.confs.FindByUserID(ctx, userID)
	if err != nil {
		// 原 GORM Preload 链路中，配置查询失败会使整个用户详情查询失败。
		return nil, apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	var (
		counts UserRelationCounts
		stats  UserVideoStats
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		counts = s.relations.RelationCounts(groupCtx, userID, false)

		return nil
	})
	group.Go(func() error {
		stats = s.videoStats.AggregateByAuthor(groupCtx, user.Username)

		return nil
	})
	_ = group.Wait()

	return &UserDetailResult{
		ID:          user.ID,
		Username:    user.Username,
		Avatar:      user.Avatar,
		BannerURL:   s.ResolveUserBannerURL(ctx, conf),
		Description: user.Description,
		Gender:      user.Gender,
		Birthday:    domain.FormatBirthday(user.Birthday),
		Level:       user.Level,
		Exp:         user.Exp,
		FollowCount: counts.FocusCount,
		FansCount:   counts.FansCount,
		TotalLikes:  stats.TotalLikes,
		TotalViews:  stats.TotalViews,
		LikeTags:    resolveLikeTags(conf),
	}, nil
}

// AdminUserInfo 后台当前登录用户信息（含角色映射）。
func (s *ProfileService) AdminUserInfo(ctx context.Context, userID uint) (*AdminUserInfoResult, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	roleMap, err := s.roles.RoleMap(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	counts := s.relations.RelationCounts(ctx, userID, false)

	return &AdminUserInfoResult{
		ID:             user.ID,
		Username:       user.Username,
		Email:          user.Email,
		Avatar:         user.Avatar,
		Description:    user.Description,
		Gender:         user.Gender,
		Birthday:       domain.FormatBirthday(user.Birthday),
		Level:          user.Level,
		Exp:            user.Exp,
		CoinCount:      user.CoinCount,
		FollowCount:    counts.FocusCount,
		FansCount:      counts.FansCount,
		DynamicCount:   s.relations.DynamicCount(ctx, userID),
		CurrentRoleID:  user.CurrentRoleID,
		RegisterSource: user.RegisterSource,
		RoleMap:        roleMap,
	}, nil
}

// UpdateProfile 更新用户资料；改名时在同一事务内级联更新视频侧的作者用户名。
//
// 用户侧与管理端共用同一实现（原两个 handler 的逻辑逐行相同）。
func (s *ProfileService) UpdateProfile(ctx context.Context, in UpdateProfileInput) error {
	user, err := s.users.FindByID(ctx, in.UserID)
	if err != nil || user == nil {
		return apperr.Wrap("application/identity/profile_service", domain.ErrUserNotFound, err)
	}

	changes, err := s.buildProfileChanges(ctx, in, user)
	if err != nil {
		return err
	}

	if changes.IsEmpty() {
		return domain.ErrNoFieldsToUpdate
	}

	usernameChanged := changes.Username != nil
	oldUsername := user.Username

	if usernameChanged {
		newUsername := *changes.Username

		if err = s.txm.Do(ctx, func(txCtx context.Context) error {
			if updateErr := s.users.UpdateProfile(txCtx, in.UserID, changes); updateErr != nil {
				return updateErr
			}

			return s.renamer.RenameAuthor(txCtx, oldUsername, newUsername)
		}); err != nil {
			return apperr.Wrap("application/identity/profile_service", ErrUpdateUserFailed, err)
		}
	} else if err = s.users.UpdateProfile(ctx, in.UserID, changes); err != nil {
		return apperr.Wrap("application/identity/profile_service", ErrUpdateUserFailed, err)
	}

	s.invalidateAfterProfileUpdate(ctx, in.UserID, oldUsername, changes)

	return nil
}

// buildProfileChanges 把入参翻译成领域侧的部分更新集合，并做占用/取值校验。
func (s *ProfileService) buildProfileChanges(
	ctx context.Context,
	in UpdateProfileInput,
	user *domain.User,
) (domain.ProfileChanges, error) {
	var changes domain.ProfileChanges

	if in.Username != "" {
		// 与原实现一致：统计失败时按「未占用」继续（唯一索引兜底）。
		count, _ := s.users.CountByUsernameExcluding(ctx, in.Username, in.UserID)
		if count > 0 {
			return changes, domain.ErrUsernameExists
		}

		if in.Username != user.Username {
			username := in.Username
			changes.Username = &username
		}
	}

	if in.Avatar != "" {
		avatar := in.Avatar
		changes.Avatar = &avatar
	}

	if in.Description != "" {
		description := in.Description
		changes.Description = &description
	}

	if in.Gender != nil {
		if err := domain.ValidateGender(*in.Gender); err != nil {
			return changes, err
		}

		gender := *in.Gender
		changes.Gender = &gender
	}

	if in.Birthday != "" {
		birthday, err := domain.ParseBirthday(in.Birthday)
		if err != nil {
			return changes, err
		}

		changes.Birthday = &birthday
	}

	return changes, nil
}

// invalidateAfterProfileUpdate 资料变更后失效用户名派生缓存与视频详情缓存。
//
// 与原实现一致：用户名/头像/简介会嵌入视频详情 meta，变更后需批量失效；
// 改名还会动 author_username，需连同 authorID 缓存一起清。
func (s *ProfileService) invalidateAfterProfileUpdate(
	ctx context.Context,
	userID uint,
	oldUsername string,
	changes domain.ProfileChanges,
) {
	if s.caches == nil {
		return
	}

	s.caches.InvalidateUsername(userID)

	usernameChanged := changes.Username != nil
	displayChanged := usernameChanged || changes.Avatar != nil || changes.Description != nil

	if !displayChanged {
		return
	}

	authorUsername := oldUsername
	if usernameChanged && *changes.Username != "" {
		authorUsername = *changes.Username
	}

	s.caches.InvalidateAuthorVideos(ctx, authorUsername, usernameChanged)
}

// ResolveUserBannerURL 解析用户主页横幅地址：自定义 URL 优先，其次系统横幅。
//
// 等价原 api/user_api/user_banner_helper.go 的 resolveUserBannerURL。
func (s *ProfileService) ResolveUserBannerURL(ctx context.Context, conf *domain.UserConf) string {
	if conf != nil {
		if url := strings.TrimSpace(conf.BannerURL); url != "" {
			return url
		}
	}

	bannerID := domain.ResolveDefaultUserBannerID(s.configuredDefaultBannerID())
	if conf != nil && conf.BannerID > 0 {
		bannerID = conf.BannerID
	}

	if bannerID == 0 {
		return ""
	}

	banner, err := s.banners.FindVisible(ctx, bannerID)
	if err != nil || banner == nil {
		return ""
	}

	return banner.Cover
}

func (s *ProfileService) configuredDefaultBannerID() uint {
	cfg := s.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return 0
	}

	return cfg.Site.DefaultUserBannerID
}

// resolveLikeTags 归一化用户喜欢的标签；配置缺失时返回空切片而非 nil。
func resolveLikeTags(conf *domain.UserConf) []string {
	if conf == nil {
		return []string{}
	}

	return domain.NormalizeLikeTags(conf.LikeTags)
}
