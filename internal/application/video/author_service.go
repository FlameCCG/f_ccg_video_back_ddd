package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// AuthorService 视频作者 ID 解析与视频缓存失效用例服务
// （原 service/video_service/author_cache.go）。
//
// 该服务同时是 video 上下文对外暴露的缓存失效入口
// （identity 更新用户资料后需要失效视频详情 meta 与作者 ID 缓存）。
type AuthorService struct {
	videos    domain.Repository
	authorIDs AuthorIDCache
	meta      DetailMetaCache
	usernames UsernameCache
	logger    *zap.Logger
}

// NewAuthorService 构造作者解析与缓存失效服务。
func NewAuthorService(
	videos domain.Repository,
	authorIDs AuthorIDCache,
	meta DetailMetaCache,
	usernames UsernameCache,
	logger *zap.Logger,
) *AuthorService {
	return &AuthorService{videos: videos, authorIDs: authorIDs, meta: meta, usernames: usernames, logger: logger}
}

// AuthorID 获取视频作者 userID。
//
// 查找顺序与原 GetVideoAuthorID 一致：作者 ID 缓存 → 详情 meta 缓存 → DB。
func (s *AuthorService) AuthorID(ctx context.Context, videoID uint) (uint, error) {
	if videoID == 0 {
		return 0, domain.ErrVideoIDRequired
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	// 先读 authorId 缓存
	if authorID, ok := s.authorIDs.Load(ctx, videoID); ok && authorID > 0 {
		return authorID, nil
	}

	// 复用详情 meta 缓存（避免 DB）
	if meta, ok := s.meta.Load(ctx, videoID); ok && meta != nil && meta.Author.ID > 0 {
		s.authorIDs.Store(ctx, videoID, meta.Author.ID)

		return meta.Author.ID, nil
	}

	// 回退 DB：videos.author_username -> users.id
	authorID, err := s.videos.AuthorIDByVideoID(ctx, videoID)
	if err != nil {
		return 0, err
	}

	if authorID == 0 {
		return 0, domain.ErrVideoAuthorNotFound
	}

	s.authorIDs.Store(ctx, videoID, authorID)

	return authorID, nil
}

// InvalidateVideoMetaCache 仅失效详情 meta 缓存（标题/封面/状态等），不影响 stats。
func (s *AuthorService) InvalidateVideoMetaCache(ctx context.Context, videoID uint) {
	if videoID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	// 删除并同步 bump generation，阻止在途旧 DB 快照回填。
	s.meta.Invalidate(ctx, videoID)
}

// InvalidateVideoAuthorIDCache 仅失效作者 ID 缓存。
func (s *AuthorService) InvalidateVideoAuthorIDCache(ctx context.Context, videoID uint) {
	if videoID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	s.authorIDs.Invalidate(ctx, videoID)
}

// InvalidateVideoAuthorCache 失效 meta + authorID。
//
// 禁止连带 clear stats（未落库增量会永久丢失）：
// 普通更新/改状态/删除/恢复均走此路径；仅极少数 stats 重建场景才调用 InvalidateStatsCache。
func (s *AuthorService) InvalidateVideoAuthorCache(ctx context.Context, videoID uint) {
	s.InvalidateVideoMetaCache(ctx, videoID)
	s.InvalidateVideoAuthorIDCache(ctx, videoID)
}

// InvalidateVideoMetaCacheByPartition 分区名/图标变更后，批量失效该分区下视频详情 meta。
// 不碰 stats。
func (s *AuthorService) InvalidateVideoMetaCacheByPartition(ctx context.Context, partitionID uint) {
	if partitionID == 0 {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	videoIDs, err := s.videos.IDsByPartition(ctx, partitionID)
	if err != nil {
		s.logger.Warn("list videos by partition for meta invalidate failed",
			zap.Uint("partitionId", partitionID), zap.Error(err))

		return
	}

	for _, videoID := range videoIDs {
		s.InvalidateVideoMetaCache(ctx, videoID)
	}
}

// InvalidateVideosByAuthor 批量失效该作者名下视频的详情缓存。
//
// 与原 api/user_api/update_user_info.go 的收尾逻辑一致：
// 用户名/头像/简介会嵌入视频详情 meta，变更后需批量失效；
// usernameChanged 为 true 时连同 authorID 缓存一起清（author_username 也变了）。
func (s *AuthorService) InvalidateVideosByAuthor(ctx context.Context, authorUsername string, usernameChanged bool) {
	if authorUsername == "" {
		return
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	videoIDs, err := s.videos.IDsByAuthor(ctx, authorUsername)
	if err != nil {
		s.logger.Warn("list videos by author for cache invalidate failed",
			zap.String("authorUsername", authorUsername), zap.Error(err))

		return
	}

	for _, videoID := range videoIDs {
		if usernameChanged {
			s.InvalidateVideoAuthorCache(ctx, videoID)
		} else {
			s.InvalidateVideoMetaCache(ctx, videoID)
		}
	}
}

// InvalidateUserUsernameCache 失效用户名派生缓存（改名/资料更新后调用）。
func (s *AuthorService) InvalidateUserUsernameCache(userID uint) {
	if userID == 0 {
		return
	}

	s.usernames.Delete(userID)
}

// UsernameByID 读取用户名：优先进程内缓存，再查 DB。
//
// 与原 fetchUsernameByID 的前两级一致；原实现的第三级「从 gin.Context 兜底」
// 依赖 gin.Context 被当作 context.Context 传入，新架构 gin.Context 止步 handler，
// 该兜底不再可达。
func (s *AuthorService) UsernameByID(ctx context.Context, userID uint) string {
	if userID == 0 {
		return ""
	}

	if username, ok := s.usernames.Load(userID); ok {
		return username
	}

	username, err := s.videos.UsernameByUserID(ctx, userID)
	if err != nil || username == "" {
		return ""
	}

	s.usernames.Store(userID, username)

	return username
}
