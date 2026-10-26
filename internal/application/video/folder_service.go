package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// FolderService 收藏夹用例服务（原 service/video_service/folder_service.go）。
type FolderService struct {
	txm      tx.Manager
	folders  domain.FolderRepository
	query    FolderQuery
	cache    FolderListCache
	stats    *StatsService
	authors  *AuthorService
	creators CreatorAnalytics
}

// NewFolderService 构造收藏夹用例服务。
func NewFolderService(
	txm tx.Manager,
	folders domain.FolderRepository,
	query FolderQuery,
	cache FolderListCache,
	stats *StatsService,
	authors *AuthorService,
	creators CreatorAnalytics,
) *FolderService {
	return &FolderService{
		txm:      txm,
		folders:  folders,
		query:    query,
		cache:    cache,
		stats:    stats,
		authors:  authors,
		creators: creators,
	}
}

// EnsureDefaultFolder 读取用户默认收藏夹，缺失时创建。
//
// 该方法同时是 video 上下文对外暴露的能力（identity 登录成功后会调用，
// 保证老用户也有默认收藏夹）。
func (s *FolderService) EnsureDefaultFolder(ctx context.Context, userID uint) (*domain.FavoriteFolder, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	folder, err := s.folders.FindDefault(ctx, userID)
	if err != nil {
		return nil, err
	}

	if folder != nil {
		return folder, nil
	}

	created := &domain.FavoriteFolder{
		UserID:    userID,
		Name:      domain.DefaultFolderName,
		IsDefault: true,
	}
	if createErr := s.folders.Create(ctx, created); createErr != nil {
		return nil, createErr
	}

	s.cache.Invalidate(userID)

	return created, nil
}

// CreateFolder 创建收藏夹。
func (s *FolderService) CreateFolder(ctx context.Context, userID uint, name string) (*domain.FavoriteFolder, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if err := domain.ValidateFolderName(name); err != nil {
		return nil, err
	}

	folder := &domain.FavoriteFolder{UserID: userID, Name: name}
	if err := s.folders.Create(ctx, folder); err != nil {
		return nil, err
	}

	s.cache.Invalidate(userID)

	return folder, nil
}

// UpdateFolder 更新收藏夹信息（目前仅名称）。
func (s *FolderService) UpdateFolder(ctx context.Context, userID, folderID uint, name string) (*domain.FavoriteFolder, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	folder, err := s.folders.FindByIDAndUser(ctx, userID, folderID)
	if err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			return nil, domain.ErrFolderNotFoundOrForbidden
		}

		return nil, err
	}

	if err := folder.Rename(name); err != nil {
		return nil, err
	}

	if err := s.folders.Update(ctx, folder); err != nil {
		return nil, err
	}

	s.cache.Invalidate(userID)

	return folder, nil
}

// DeleteFolder 删除收藏夹并同步收藏计数副作用。
func (s *FolderService) DeleteFolder(ctx context.Context, userID, folderID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	folder, err := s.folders.FindByIDAndUser(ctx, userID, folderID)
	if err != nil {
		return err
	}

	if err := folder.CanDelete(); err != nil {
		return err
	}

	var affectedVideoIDs []uint

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		ids, txErr := s.folders.DeleteWithVideos(txCtx, userID, folder)
		if txErr != nil {
			return txErr
		}

		affectedVideoIDs = ids

		return nil
	}); err != nil {
		return err
	}

	s.cache.Invalidate(userID)
	s.syncFavoriteSideEffectsAfterFolderDelete(ctx, affectedVideoIDs)

	return nil
}

// GetFolder 读取用户名下的收藏夹。
func (s *FolderService) GetFolder(ctx context.Context, userID, folderID uint) (*domain.FavoriteFolder, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	return s.folders.FindByIDAndUser(ctx, userID, folderID)
}

// ListFolders 获取用户收藏夹列表及视频数量。
func (s *FolderService) ListFolders(ctx context.Context, userID, videoID uint) ([]FolderWithCount, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if cached, ok := s.cache.Load(userID, videoID); ok {
		return cached, nil
	}

	rows, err := s.query.ListWithCount(ctx, userID, videoID)
	if err != nil {
		return nil, err
	}

	s.cache.Store(userID, videoID, rows)

	return rows, nil
}

// ListFolderVideos 获取收藏夹内视频列表。
func (s *FolderService) ListFolderVideos(
	ctx context.Context,
	userID, folderID uint,
	page, pageSize, sort int,
) ([]FolderVideoItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = 20
	}

	if pageSize > 50 {
		pageSize = 50
	}

	if _, err := s.folders.FindByIDAndUser(ctx, userID, folderID); err != nil {
		return nil, 0, err
	}

	return s.query.ListVideos(ctx, folderID, page, pageSize, domain.NormalizeFolderVideoSort(sort))
}

// resolveFolder 解析目标收藏夹：folderID 为 0 时用默认夹。
func (s *FolderService) resolveFolder(ctx context.Context, userID, folderID uint) (*domain.FavoriteFolder, error) {
	if folderID == 0 {
		return s.EnsureDefaultFolder(ctx, userID)
	}

	return s.GetFolder(ctx, userID, folderID)
}

// syncFavoriteSideEffectsAfterFolderDelete 删夹后同步热度、创作中心指标与缓存计数。
func (s *FolderService) syncFavoriteSideEffectsAfterFolderDelete(ctx context.Context, videoIDs []uint) {
	s.refreshFavoriteStatsCache(ctx, videoIDs)

	for _, videoID := range videoIDs {
		s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreFavoriteWeight, -1)

		if authorID, err := s.authors.AuthorID(ctx, videoID); err == nil {
			_ = s.creators.IncrFavorites(ctx, authorID, -1)
		}
	}
}

// refreshFavoriteStatsCache 事务已把 MySQL favorite_count 减过一次。
// 热缓存仅 patch 已存在 key；冷缓存从 DB 读权威值，禁止再 delta（否则双减）。
func (s *FolderService) refreshFavoriteStatsCache(ctx context.Context, videoIDs []uint) {
	if len(videoIDs) == 0 {
		return
	}

	ctx = orTODO(ctx)

	for _, videoID := range videoIDs {
		if _, err := s.stats.ApplyDeltaIfCached(ctx, videoID, domain.StatsDelta{FavoriteCount: -1}); err != nil {
			// 失败不 wipe；日志可见即可
			continue
		}
	}
}
