package comment

import (
	"context"
	"time"

	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	commentrepo "f_ccg_video_back_ddd/internal/domain/comment/repository"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// likeCacheDBTimeout 点赞缓存重建时读库与回写缓存的超时，为 2 秒。
const likeCacheDBTimeout = 2 * time.Second

// ensureLikeCacheReady 点赞热路径：缓存不完整则查 DB。
//   - DB 有赞：全量灌 set+count
//   - DB=0：只清半残 key，不先置 count=0；由本次 Lua 用当前操作初始化并入队
func (s *LikeCacheCoordinator) ensureLikeCacheReady(ctx context.Context, commentID uint) error {
	ok, err := s.likeCache.Complete(ctx, commentID)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeCacheDBTimeout)
	defer cancel()

	userIDs, err := s.likes.UserIDsByComment(cacheCtx, commentID)
	if err != nil {
		return err
	}

	if len(userIDs) > 0 {
		return s.likeCache.Write(cacheCtx, commentID, userIDs)
	}

	// DB=0：不写 count=0，清掉半残后交给 toggle Lua（GET miss 当 0 + 当前 SADD/入队）
	return s.likeCache.Clear(cacheCtx, commentID)
}

// rebuildLikeCache 以 comment_likes 为准全量重建 set+count（同 TTL）。
// 读路径 / 异步重建使用：DB=0 时会写入 count=0，避免列表反复 miss。
func (s *LikeCacheCoordinator) rebuildLikeCache(ctx context.Context, commentID uint) error {
	if !s.likeCache.Available() || commentID == 0 {
		return commenterrors.ErrLikeCacheUnavailable
	}

	cacheCtx, cancel := context.WithTimeout(ctx, likeCacheDBTimeout)
	defer cancel()

	userIDs, err := s.likes.UserIDsByComment(cacheCtx, commentID)
	if err != nil {
		return err
	}

	return s.likeCache.Write(cacheCtx, commentID, userIDs)
}

// scheduleLikeCacheRebuild 读路径 miss 时异步全量重建，不阻塞响应。
func (s *LikeCacheCoordinator) scheduleLikeCacheRebuild(parent context.Context, commentIDs ...uint) {
	if !s.likeCache.Available() || len(commentIDs) == 0 {
		return
	}

	// 去重
	seen := make(map[uint]struct{}, len(commentIDs))
	ids := make([]uint, 0, len(commentIDs))

	for _, id := range commentIDs {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return
	}

	s.tasks.Go(parent, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
		for _, id := range ids {
			if err := s.rebuildLikeCache(asyncCtx, id); err != nil {
				s.logger.Warn("async rebuild comment like cache failed", zap.Uint("commentId", id), zap.Error(err))
			}
		}
	})
}

// LikeCacheCoordinator 负责点赞缓存完整性与重建。
type LikeCacheCoordinator struct {
	likes     commentrepo.LikeRepository
	likeCache LikeCache
	tasks     *ctxutil.DetachedTaskGroup
	logger    *zap.Logger
}

func NewLikeCacheCoordinator(likes commentrepo.LikeRepository, likeCache LikeCache, tasks *ctxutil.DetachedTaskGroup, logger *zap.Logger) *LikeCacheCoordinator {
	return &LikeCacheCoordinator{likes: likes, likeCache: likeCache, tasks: tasks, logger: logger}
}
