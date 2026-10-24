package video

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/coinutil"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// Triple 一键三连：点赞/收藏/投币在同一 MySQL 事务内 ensure（真原子）。
//
// 展示计数在事务提交后统一更新；Redis 失败走 committed 兜底（DB + 失效缓存）。
// 跨上下文的作者硬币奖励通过 CoinLedger 端口在同一事务内完成（见 decisions.md D-010）。
func (s *InteractionService) Triple(
	ctx context.Context,
	videoID, userID, folderID uint,
	coins int64,
) (*TripleResult, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	coins = domain.NormalizeTripleCoins(coins)

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		return nil, err
	}

	ctx = orTODO(ctx)

	// 收藏夹在事务外解析/创建默认夹，避免拉长业务事务
	folder, err := s.folder.resolveFolder(ctx, userID, folderID)
	if err != nil {
		return nil, err
	}

	var applied domain.TripleOutcome

	err = s.txm.Do(ctx, func(txCtx context.Context) error {
		res, txErr := s.applyTriple(txCtx, videoID, userID, folder.ID, coins)
		if txErr != nil {
			return txErr
		}

		applied = *res

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 提交后统一写展示计数
	statsDelta := domain.StatsDelta{
		Likes:         applied.LikeDelta,
		FavoriteCount: applied.FavDelta,
		CoinCount:     applied.CoinDelta,
	}

	stats, statsErr := s.stats.ApplyCommittedDelta(ctx, videoID, statsDelta)
	if statsErr != nil {
		s.logger.Error("triple committed but stats update failed",
			zap.Uint("videoId", videoID),
			zap.Uint("userId", userID),
			zap.Error(statsErr),
		)

		if stats == nil {
			stats, _ = s.stats.Load(ctx, videoID)
		}
	}

	if stats == nil {
		stats = &domain.Stats{}
	}

	// 副作用：热度 / 分析 / 经验 / 通知（失败不影响三连成功）
	if applied.LikeDelta != 0 {
		s.recordLikeHotScoreAsync(ctx, videoID, applied.LikeDelta)
	}

	if applied.FavDelta != 0 {
		s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreFavoriteWeight, applied.FavDelta)

		if applied.AuthorID > 0 {
			_ = s.creators.IncrFavorites(ctx, applied.AuthorID, applied.FavDelta)
		}
	}

	if applied.CoinDelta > 0 {
		s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreCoinWeight, applied.CoinDelta)

		if applied.AuthorID > 0 {
			_ = s.creators.IncrCoins(ctx, applied.AuthorID, applied.CoinDelta)
		}

		_ = s.exp.CoinExpForVideo(ctx, userID, videoID, applied.CoinDelta)
	}

	if applied.NewlyLiked {
		s.awardLikeExpAsync(ctx, userID)
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(notifyCtx context.Context) {
			s.sendVideoLikeNotification(notifyCtx, userID, videoID)
		})
	}

	if applied.NewlyFaved || applied.FavDelta != 0 {
		s.folderC.Invalidate(userID)
	}

	return &TripleResult{
		Likes:         stats.Likes,
		CoinCount:     stats.CoinCount,
		FavoriteCount: stats.FavoriteCount,
		IsLiked:       true, // ensure 语义：成功后必为已赞（原本已赞或本次新赞）
		IsFavorited:   true,
		IsCoined:      applied.NewlyCoined || applied.CoinDelta > 0 || s.HasCoined(ctx, videoID, userID),
	}, nil
}

// applyTriple 在同一事务内 ensure 赞/藏/币。
func (s *InteractionService) applyTriple(
	ctx context.Context,
	videoID, userID, folderID uint,
	coins int64,
) (*domain.TripleOutcome, error) {
	out := &domain.TripleOutcome{}

	authorID, err := s.videos.PublishedAuthorIDByVideoID(ctx, videoID)
	if err != nil {
		return nil, err
	}

	out.AuthorID = authorID

	if err := s.ledger.LockAccounts(ctx, []uint{userID, authorID}); err != nil {
		return nil, err
	}

	// --- 点赞 ensure（只加不减）---
	if err := s.likes.Create(ctx, videoID, userID); err != nil {
		if !errors.Is(err, domain.ErrDuplicateLike) {
			return nil, err
		}
		// 已赞
	} else {
		out.LikeDelta = 1
		out.NewlyLiked = true
	}

	// --- 收藏 ensure ---
	existsAny, err := s.folders.UserFavorited(ctx, videoID, userID)
	if err != nil {
		return nil, err
	}

	created, err := s.folders.EnsureLink(ctx, folderID, videoID)
	if err != nil {
		return nil, err
	}

	// 首次被该用户收藏：涨 favorite_count + 作者奖励
	if created && !existsAny {
		out.FavDelta = 1
		out.NewlyFaved = true

		if authorID > 0 && authorID != userID {
			key := favoriteRewardKey(userID, videoID)
			if err := s.ledger.ApplyChange(
				ctx,
				authorID,
				domain.FavoriteAuthorReward,
				"作品被收藏 +"+coinutil.Format(domain.FavoriteAuthorReward),
				&key,
			); err != nil {
				return nil, err
			}
		}
	}

	// --- 投币 ensure ---
	coinErr := s.coins.LockByVideoAndUser(ctx, videoID, userID)

	switch {
	case coinErr == nil:
		// 已投过：三连幂等成功，不加币
		out.NewlyCoined = true
	case !isRecordNotFound(coinErr):
		return nil, coinErr
	default:
		// 未投：在事务内投币（用户与作者已在上方按固定顺序锁定，无需重复加锁）
		delta, err := s.applyCoinWithoutRelock(ctx, videoID, userID, authorID, coins)
		if err != nil {
			return nil, err
		}

		out.CoinDelta = delta
		out.NewlyCoined = delta > 0
	}

	return out, nil
}

// applyCoinWithoutRelock 假定 user/author 已在事务内锁定。
func (s *InteractionService) applyCoinWithoutRelock(
	ctx context.Context,
	videoID, userID, authorID uint,
	coins int64,
) (int64, error) {
	add, err := domain.ClampCoins(coins)
	if err != nil {
		return 0, err
	}

	if createErr := s.coins.Create(ctx, videoID, userID, add); createErr != nil {
		if errors.Is(createErr, domain.ErrDuplicateCoin) {
			return 0, nil
		}

		return 0, createErr
	}

	if err := s.chargeCoins(ctx, videoID, userID, authorID, add); err != nil {
		return 0, err
	}

	return add, nil
}
