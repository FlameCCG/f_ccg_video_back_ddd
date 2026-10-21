package adapters

import (
	"context"
	"errors"
	"io"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	searchapp "f_ccg_video_back_ddd/internal/application/search"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/casbins"
)

// IdentityVideoQuery 把 video 上下文的用户投稿用例适配为 identity 声明的 UserVideoQuery 端口。
//
// identity 的端口以 any 返回读模型，避免 identity 反向依赖 video 的 DTO 类型。
type IdentityVideoQuery struct {
	videos *videoapp.UserVideoService
}

var _ identityapp.UserVideoQuery = (*IdentityVideoQuery)(nil)

// NewIdentityVideoQuery 构造用户投稿查询适配器。
func NewIdentityVideoQuery(videos *videoapp.UserVideoService) *IdentityVideoQuery {
	return &IdentityVideoQuery{videos: videos}
}

// ListUserVideos 用户主页视频分页列表。
func (a *IdentityVideoQuery) ListUserVideos(
	ctx context.Context,
	userID, viewerID uint,
	page, pageSize, sort, auditStatus int,
) (any, int64, error) {
	items, total, err := a.videos.ListUserVideos(ctx, userID, viewerID, page, pageSize, sort, auditStatus)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// RecentActions 最近点赞/投币的视频列表。
func (a *IdentityVideoQuery) RecentActions(
	ctx context.Context,
	userID uint,
	action string,
	limit int,
) (any, int, error) {
	items, err := a.videos.RecentActions(ctx, userID, action, limit)
	if err != nil {
		return nil, 0, err
	}

	return items, len(items), nil
}

// IdentityVideoCacheInvalidator 把 video 上下文的缓存失效能力适配为 identity 的端口。
//
// 用户改名 / 资料更新后需要连带失效「userID → username」与该作者名下视频的详情缓存。
type IdentityVideoCacheInvalidator struct {
	authors *videoapp.AuthorService
}

var _ identityapp.VideoCacheInvalidator = (*IdentityVideoCacheInvalidator)(nil)

// NewIdentityVideoCacheInvalidator 构造视频缓存失效适配器。
func NewIdentityVideoCacheInvalidator(authors *videoapp.AuthorService) *IdentityVideoCacheInvalidator {
	return &IdentityVideoCacheInvalidator{authors: authors}
}

// InvalidateUsername 失效「userID → username」派生缓存。
func (a *IdentityVideoCacheInvalidator) InvalidateUsername(userID uint) {
	a.authors.InvalidateUserUsernameCache(userID)
}

// InvalidateAuthorVideos 批量失效该作者名下视频的详情缓存。
func (a *IdentityVideoCacheInvalidator) InvalidateAuthorVideos(
	ctx context.Context,
	authorUsername string,
	usernameChanged bool,
) {
	a.authors.InvalidateVideosByAuthor(ctx, authorUsername, usernameChanged)
}

// IdentityCoinLedger 把 identity 的硬币用例适配为 video 声明的 CoinLedger 端口。
//
// 端口刻意不共享哨兵错误，改为暴露 IsInsufficient 判定，避免 video 反向 import identity 领域包。
type IdentityCoinLedger struct {
	coins *identityapp.CoinService
}

var _ videoapp.CoinLedger = (*IdentityCoinLedger)(nil)

// NewIdentityCoinLedger 构造硬币账本适配器。
func NewIdentityCoinLedger(coins *identityapp.CoinService) *IdentityCoinLedger {
	return &IdentityCoinLedger{coins: coins}
}

// LockAccounts 把账户不存在的错误翻译为 video 上下文的错误契约。
func (a *IdentityCoinLedger) LockAccounts(ctx context.Context, userIDs []uint) error {
	err := a.coins.LockAccounts(ctx, userIDs)
	if errors.Is(err, identitydomain.ErrUserRecordNotFound) {
		return videodomain.ErrRecordNotFound
	}
	return err
}

// ApplyChange 增减用户硬币余额并写入流水。
func (a *IdentityCoinLedger) ApplyChange(
	ctx context.Context,
	userID uint,
	delta float64,
	reason string,
	dedupKey *string,
) error {
	_, _, err := a.coins.ApplyChange(ctx, userID, delta, reason, dedupKey)

	return err
}

// IsInsufficient 判断错误是否为「硬币余额不足」。
func (a *IdentityCoinLedger) IsInsufficient(err error) bool {
	return identitydomain.IsCoinInsufficient(err)
}

// IdentityPublishedVideoStats 把 video 的作者统计读模型映射为 identity 端口模型。
type IdentityPublishedVideoStats struct {
	stats *videoapp.AuthorStatsService
}

var _ identityapp.PublishedVideoStatsQuery = (*IdentityPublishedVideoStats)(nil)

// NewIdentityPublishedVideoStats 构造作者统计适配器。
func NewIdentityPublishedVideoStats(stats *videoapp.AuthorStatsService) *IdentityPublishedVideoStats {
	return &IdentityPublishedVideoStats{stats: stats}
}

// AggregateByAuthor 汇总作者已发布作品的权威播放、点赞计数。
func (a *IdentityPublishedVideoStats) AggregateByAuthor(
	ctx context.Context,
	authorUsername string,
) identityapp.UserVideoStats {
	stats := a.stats.AggregateByAuthor(ctx, authorUsername)

	return identityapp.UserVideoStats{
		TotalLikes: stats.TotalLikes,
		TotalViews: stats.TotalViews,
	}
}

// IdentityVideoTagWriter 把视频聚合的标签写入能力适配为 identity 的弱类型响应端口。
type IdentityVideoTagWriter struct {
	videos videodomain.VideoRepository
}

var _ identityapp.VideoTagWriter = (*IdentityVideoTagWriter)(nil)

// NewIdentityVideoTagWriter 构造标签写入适配器。
func NewIdentityVideoTagWriter(videos videodomain.VideoRepository) *IdentityVideoTagWriter {
	return &IdentityVideoTagWriter{videos: videos}
}

// CreateOrGetTag 创建或读取同名标签。
func (a *IdentityVideoTagWriter) CreateOrGetTag(ctx context.Context, name string) (any, error) {
	return a.videos.CreateOrGetTag(ctx, name)
}

// IdentitySearchQuery 把 search 上下文的强类型读模型适配为 identity 的接口响应端口。
type IdentitySearchQuery struct {
	search   *searchapp.SearchService
	mentions *searchapp.MentionService
}

var (
	_ identityapp.UserSearchQuery     = (*IdentitySearchQuery)(nil)
	_ identityapp.MentionSuggestQuery = (*IdentitySearchQuery)(nil)
)

// NewIdentitySearchQuery 构造身份侧搜索适配器。
func NewIdentitySearchQuery(
	search *searchapp.SearchService,
	mentions *searchapp.MentionService,
) *IdentitySearchQuery {
	return &IdentitySearchQuery{search: search, mentions: mentions}
}

// SearchUsers 搜索用户。
func (a *IdentitySearchQuery) SearchUsers(
	ctx context.Context,
	params identityapp.UserSearchParams,
) (any, error) {
	return a.search.SearchUsers(ctx, searchapp.SearchOptions{
		Keyword:   params.Keyword,
		Page:      params.Page,
		PageSize:  params.PageSize,
		UserSort:  params.UserSort,
		UserOrder: params.UserOrder,
	})
}

// SuggestFollowUsers 返回当前用户关注列表中的 @ 候选。
func (a *IdentitySearchQuery) SuggestFollowUsers(
	ctx context.Context,
	userID uint,
	keyword string,
	page, pageSize int,
) (any, int64, error) {
	return a.mentions.SuggestFollowUsers(ctx, userID, keyword, page, pageSize)
}

// IdentityBannerStorage 把 media 存储解析器适配为用户横幅存储端口。
type IdentityBannerStorage struct {
	resolver mediadomain.StorageResolver
}

var _ identityapp.BannerStorage = (*IdentityBannerStorage)(nil)

// NewIdentityBannerStorage 构造横幅存储适配器。
func NewIdentityBannerStorage(resolver mediadomain.StorageResolver) *IdentityBannerStorage {
	return &IdentityBannerStorage{resolver: resolver}
}

// ValidateUploadID 校验上传标识。
func (a *IdentityBannerStorage) ValidateUploadID(uploadID string) error {
	return a.resolver.ValidateUploadID(uploadID)
}

// EnsureReady 校验当前存储驱动可解析。
func (a *IdentityBannerStorage) EnsureReady() error {
	_, err := a.resolver.Resolve()

	return err
}

// SaveBanner 保存用户横幅。
func (a *IdentityBannerStorage) SaveBanner(
	ctx context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	contentType string,
) (string, error) {
	driver, err := a.resolver.Resolve()
	if err != nil {
		return "", err
	}

	return driver.SaveFile(ctx, relativePath, size, reader, contentType)
}

// IdentityUserRoleBinder 把 RBAC Enforcer 适配为 identity 的用户角色写入端口。
type IdentityUserRoleBinder struct {
	enforcer rbacapp.Enforcer
}

var _ identityapp.UserRoleBinder = (*IdentityUserRoleBinder)(nil)

// NewIdentityUserRoleBinder 构造用户角色绑定适配器。
func NewIdentityUserRoleBinder(enforcer rbacapp.Enforcer) *IdentityUserRoleBinder {
	return &IdentityUserRoleBinder{enforcer: enforcer}
}

// AddRoleForUser 为用户添加角色。
func (a *IdentityUserRoleBinder) AddRoleForUser(userID uint, roleName string) error {
	_, err := a.enforcer.AddRoleForUser(casbins.GetUserSub(userID), roleName)

	return err
}

// IdentityDefaultFolderEnsurer 把 video 收藏夹用例适配为 identity 的兜底创建端口。
type IdentityDefaultFolderEnsurer struct {
	folders *videoapp.FolderService
}

var _ identityapp.DefaultFolderEnsurer = (*IdentityDefaultFolderEnsurer)(nil)

// NewIdentityDefaultFolderEnsurer 构造默认收藏夹适配器。
func NewIdentityDefaultFolderEnsurer(folders *videoapp.FolderService) *IdentityDefaultFolderEnsurer {
	return &IdentityDefaultFolderEnsurer{folders: folders}
}

// EnsureDefaultFolder 确保默认收藏夹存在。
func (a *IdentityDefaultFolderEnsurer) EnsureDefaultFolder(ctx context.Context, userID uint) error {
	_, err := a.folders.EnsureDefaultFolder(ctx, userID)

	return err
}
