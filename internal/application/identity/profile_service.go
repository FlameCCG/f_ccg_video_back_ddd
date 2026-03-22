package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/application/shared"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"golang.org/x/sync/errgroup"
)

// ProfileService 用户资料读写用例服务。
type ProfileService struct {
	cfgProvider shared.ConfigReader
	txm         tx.Manager
	users       identitydomain.UserAccountRepository
	confs       identitydomain.UserConfRepository
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
	users identitydomain.UserAccountRepository,
	confs identitydomain.UserConfRepository,
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
		return nil, identitydomain.ErrUserNotFound
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
		ID:             user.ID(),
		Username:       user.Username(),
		Email:          user.Email(),
		Avatar:         user.Avatar(),
		BannerURL:      bannerURL,
		Description:    user.Description(),
		Gender:         user.Gender(),
		Birthday:       user.FormattedBirthday(),
		Level:          user.Level(),
		Exp:            user.Exp(),
		CoinCount:      user.CoinCount(),
		FollowCount:    counts.FocusCount,
		FansCount:      counts.FansCount,
		DynamicCount:   counts.DynamicCount,
		RegisterSource: user.RegisterSource(),
		LikeTags:       conf.PublicLikeTags(),
		HasPassword:    user.HasPassword(),
		IsThirdParty:   user.IsThirdParty(),
	}, nil
}

// UserDetail 指定用户的公开详情（含作品曝光汇总）。
func (s *ProfileService) UserDetail(ctx context.Context, userID uint) (*UserDetailResult, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, identitydomain.ErrUserNotFound
	}

	conf, err := s.confs.FindByUserID(ctx, userID)
	if err != nil {
		// 关联用户配置查询失败时，整个用户详情查询失败。
		return nil, identitydomain.ErrUserNotFound
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
		stats = s.videoStats.AggregateByAuthor(groupCtx, user.Username())

		return nil
	})
	_ = group.Wait()

	return &UserDetailResult{
		ID:          user.ID(),
		Username:    user.Username(),
		Avatar:      user.Avatar(),
		BannerURL:   s.ResolveUserBannerURL(ctx, conf),
		Description: user.Description(),
		Gender:      user.Gender(),
		Birthday:    user.FormattedBirthday(),
		Level:       user.Level(),
		Exp:         user.Exp(),
		FollowCount: counts.FocusCount,
		FansCount:   counts.FansCount,
		TotalLikes:  stats.TotalLikes,
		TotalViews:  stats.TotalViews,
		LikeTags:    conf.PublicLikeTags(),
	}, nil
}

// AdminUserInfo 后台当前登录用户信息（含角色映射）。
func (s *ProfileService) AdminUserInfo(ctx context.Context, userID uint) (*AdminUserInfoResult, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return nil, identitydomain.ErrUserNotFound
	}

	roleMap, err := s.roles.RoleMap(ctx, userID)
	if err != nil {
		return nil, identitydomain.ErrUserNotFound
	}

	counts := s.relations.RelationCounts(ctx, userID, false)

	return &AdminUserInfoResult{
		ID:             user.ID(),
		Username:       user.Username(),
		Email:          user.Email(),
		Avatar:         user.Avatar(),
		Description:    user.Description(),
		Gender:         user.Gender(),
		Birthday:       user.FormattedBirthday(),
		Level:          user.Level(),
		Exp:            user.Exp(),
		CoinCount:      user.CoinCount(),
		FollowCount:    counts.FocusCount,
		FansCount:      counts.FansCount,
		DynamicCount:   s.relations.DynamicCount(ctx, userID),
		CurrentRoleID:  user.CurrentRoleID(),
		RegisterSource: user.RegisterSource(),
		RoleMap:        roleMap,
	}, nil
}

// UpdateProfile 更新用户资料；改名时在同一事务内级联更新视频侧作者用户名。
//
// 用户侧与管理端共用此实现。
func (s *ProfileService) UpdateProfile(ctx context.Context, in UpdateProfileInput) error {
	var (
		oldUsername   string
		changes       identitydomain.ProfileChanges
		validationErr error
	)
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		// 改名联动必须基于同一事务中锁定的最新用户名，避免并发请求使用同一个旧值。
		user, err := s.users.FindByIDForUpdate(txCtx, in.UserID)
		if err != nil || user == nil {
			validationErr = identitydomain.ErrUserNotFound
			return validationErr
		}
		changes, validationErr = s.buildProfileChanges(txCtx, in, user)
		if validationErr != nil {
			return validationErr
		}
		oldUsername = user.Username()
		if err := s.users.UpdateProfile(txCtx, in.UserID, changes); err != nil {
			return err
		}
		if changes.Username != nil {
			return s.renamer.RenameAuthor(txCtx, oldUsername, *changes.Username)
		}
		return nil
	})
	if validationErr != nil {
		return validationErr
	}
	if err != nil {
		return ErrUpdateUserFailed
	}
	s.invalidateAfterProfileUpdate(ctx, in.UserID, oldUsername, changes)
	return nil
}

// buildProfileChanges 把入参翻译成领域侧的部分更新集合，并做占用/取值校验。
func (s *ProfileService) buildProfileChanges(
	ctx context.Context,
	in UpdateProfileInput,
	user *identitydomain.User,
) (identitydomain.ProfileChanges, error) {
	if in.Username != "" {
		// 统计失败时按「未占用」继续（唯一索引兜底）。
		count, _ := s.users.CountByUsernameExcluding(ctx, in.Username, in.UserID)
		if err := user.EnsureUsernameAvailable(count); err != nil {
			return identitydomain.ProfileChanges{}, err
		}
	}

	return user.PlanProfile(identitydomain.ProfileInput{
		Username:    in.Username,
		Avatar:      in.Avatar,
		Description: in.Description,
		Gender:      in.Gender,
		Birthday:    in.Birthday,
	})
}

// invalidateAfterProfileUpdate 资料变更后失效用户名派生缓存与视频详情缓存。
//
// 用户名/头像/简介会嵌入视频详情 meta，变更后需批量失效；
// 改名还会动 author_username，需连同 authorID 缓存一起清。
func (s *ProfileService) invalidateAfterProfileUpdate(
	ctx context.Context,
	userID uint,
	oldUsername string,
	changes identitydomain.ProfileChanges,
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
func (s *ProfileService) ResolveUserBannerURL(ctx context.Context, conf *identitydomain.UserConf) string {
	if url := conf.CustomBannerURL(); url != "" {
		return url
	}

	bannerID := conf.SystemBannerID(s.configuredDefaultBannerID())
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
