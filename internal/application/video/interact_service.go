package video

import (
	"context"
	"errors"
	"fmt"

	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/coinutil"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// InteractionService 视频互动用例服务（播放 / 点赞 / 投币 / 收藏 / 三连）。
//
// 原 service/video_service/{interact_service.go,like_service.go,triple_service.go}。
// MySQL 账本原子；展示计数在事务提交后更新，Redis 失败则 DB 兜底并失效缓存以重建。
type InteractionService struct {
	txm      tx.Manager
	videos   domain.VideoRepository
	likes    domain.LikeRepository
	coins    domain.CoinRepository
	folders  domain.FolderRepository
	stats    *StatsService
	authors  *AuthorService
	folder   *FolderService
	folderC  FolderListCache
	creators CreatorAnalytics
	exp      ExpAwarder
	ledger   CoinLedger
	notifier VideoNotifier
	notify   NotifySourceQuery
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

// NewInteractionService 构造视频互动用例服务。
func NewInteractionService(
	txm tx.Manager,
	videos domain.VideoRepository,
	likes domain.LikeRepository,
	coins domain.CoinRepository,
	folders domain.FolderRepository,
	stats *StatsService,
	authors *AuthorService,
	folder *FolderService,
	folderCache FolderListCache,
	creators CreatorAnalytics,
	exp ExpAwarder,
	ledger CoinLedger,
	notifier VideoNotifier,
	notify NotifySourceQuery,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *InteractionService {
	return &InteractionService{
		txm:      txm,
		videos:   videos,
		likes:    likes,
		coins:    coins,
		folders:  folders,
		stats:    stats,
		authors:  authors,
		folder:   folder,
		folderC:  folderCache,
		creators: creators,
		exp:      exp,
		ledger:   ledger,
		notifier: notifier,
		notify:   notify,
		tasks:    tasks,
		logger:   logger,
	}
}

// View 浏览量 +1（不去重）；如已登录则触发「每日观看经验」（每日一次）。
func (s *InteractionService) View(ctx context.Context, videoID, userID uint) (*ViewResult, error) {
	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		return nil, err
	}

	stats, err := s.stats.ApplyDelta(ctx, videoID, domain.StatsDelta{Views: 1})
	if err != nil {
		return nil, err
	}

	s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreViewWeight, 1)

	if authorID, authorErr := s.authors.AuthorID(ctx, videoID); authorErr == nil {
		_ = s.creators.IncrViews(ctx, authorID, 1)
	}

	if userID != 0 {
		_ = s.exp.WatchExp(ctx, userID)
	}

	return &ViewResult{Views: stats.Views}, nil
}

// Coin 投币（每人每视频最多 2 个，无法撤销；已投视为幂等上限错误）。
func (s *InteractionService) Coin(ctx context.Context, videoID, userID uint, coins int64) (*CoinResult, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if coins <= 0 {
		return nil, domain.ErrCoinCountInvalid
	}

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		return nil, err
	}

	ctx = orTODO(ctx)

	var (
		delta    int64
		authorID uint
	)

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		applied, ownerID, txErr := s.applyCoin(txCtx, videoID, userID, coins)
		if txErr != nil {
			return txErr
		}

		delta = applied
		authorID = ownerID

		return nil
	})
	if err != nil {
		return nil, err
	}

	if delta > 0 {
		stats, statsErr := s.stats.ApplyCommittedDelta(ctx, videoID, domain.StatsDelta{CoinCount: delta})
		if statsErr != nil {
			// 账本已提交：不回滚硬币；尽量返回当前可读计数。
			s.logger.Error("coin committed but stats update failed",
				zap.Uint("videoId", videoID),
				zap.Uint("userId", userID),
				zap.Int64("delta", delta),
				zap.Error(statsErr),
			)

			if stats == nil {
				stats, _ = s.stats.Load(ctx, videoID)
			}

			if stats == nil {
				// 计数不可读仍视为投币成功语义弱化：返回 0 计数避免用户以为失败可重试乱扣
				return &CoinResult{CoinCount: 0}, nil
			}
		}

		s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreCoinWeight, delta)

		if authorID > 0 {
			_ = s.creators.IncrCoins(ctx, authorID, delta)
		}

		_ = s.exp.CoinExpForVideo(ctx, userID, videoID, delta)

		return &CoinResult{CoinCount: stats.CoinCount}, nil
	}

	stats, err := s.stats.Load(ctx, videoID)
	if err != nil {
		return nil, err
	}

	return &CoinResult{CoinCount: stats.CoinCount}, nil
}

// applyCoin 在事务内投币。返回本次新增币数（0 表示未新增）；已投过返回 ErrCoinLimit。
func (s *InteractionService) applyCoin(
	ctx context.Context,
	videoID, userID uint,
	coins int64,
) (delta int64, authorID uint, err error) {
	add, err := domain.ClampCoins(coins)
	if err != nil {
		return 0, 0, err
	}

	ownerID, err := s.videos.PublishedAuthorIDByVideoID(ctx, videoID)
	if err != nil {
		return 0, 0, err
	}

	// 固定加锁顺序，避免与三连/收藏并发死锁
	if err := s.ledger.LockAccounts(ctx, []uint{userID, ownerID}); err != nil {
		return 0, 0, err
	}

	lockErr := s.coins.LockByVideoAndUser(ctx, videoID, userID)
	if lockErr == nil {
		return 0, ownerID, domain.ErrCoinLimit
	}

	if !isRecordNotFound(lockErr) {
		return 0, 0, lockErr
	}

	if createErr := s.coins.Create(ctx, videoID, userID, add); createErr != nil {
		if errors.Is(createErr, domain.ErrDuplicateCoin) {
			return 0, ownerID, domain.ErrCoinLimit
		}

		return 0, 0, createErr
	}

	if err := s.chargeCoins(ctx, videoID, userID, ownerID, add); err != nil {
		return 0, 0, err
	}

	return add, ownerID, nil
}

// chargeCoins 事务内扣减投币用户余额并给作者发放奖励。
func (s *InteractionService) chargeCoins(ctx context.Context, videoID, userID, ownerID uint, add int64) error {
	if err := s.ledger.ApplyChange(
		ctx,
		userID,
		-float64(add),
		fmt.Sprintf("投币视频 %d -%s", videoID, coinutil.Format(float64(add))),
		nil,
	); err != nil {
		return err
	}

	if ownerID > 0 {
		reward := coinutil.Round(domain.CoinAuthorReward(add))
		if err := s.ledger.ApplyChange(
			ctx,
			ownerID,
			reward,
			fmt.Sprintf("作品被投币 x%d +%s", add, coinutil.Format(reward)),
			nil,
		); err != nil {
			return err
		}
	}

	return nil
}

// ToggleFavorite 收藏/取消收藏（支持 folderId，为 0 使用默认收藏夹）。
func (s *InteractionService) ToggleFavorite(
	ctx context.Context,
	videoID, userID, folderID uint,
) (*FavoriteResult, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		return nil, err
	}

	folder, err := s.folder.resolveFolder(ctx, userID, folderID)
	if err != nil {
		return nil, err
	}

	existsTarget, err := s.folders.LinkExists(ctx, folder.ID, videoID)
	if err != nil {
		return nil, err
	}

	if existsTarget {
		return s.removeFavorite(ctx, videoID, userID, folder.ID)
	}

	return s.addFavorite(ctx, videoID, userID, folder.ID)
}

func (s *InteractionService) removeFavorite(
	ctx context.Context,
	videoID, userID, folderID uint,
) (*FavoriteResult, error) {
	var deltaFavorite int64
	// 取消该收藏夹的收藏
	if err := s.folders.DeleteLink(ctx, folderID, videoID); err != nil {
		return nil, err
	}

	// 仍在其他收藏夹则不减总数
	stillFavorited, err := s.folders.UserFavorited(ctx, videoID, userID)
	if err != nil {
		return nil, err
	}

	if !stillFavorited {
		deltaFavorite = -1
	}

	s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreFavoriteWeight, deltaFavorite)

	var stats *domain.Stats

	if deltaFavorite != 0 {
		stats, err = s.stats.ApplyDelta(ctx, videoID, domain.StatsDelta{FavoriteCount: deltaFavorite})
		if err != nil {
			return nil, err
		}
	} else {
		stats, err = s.stats.Load(ctx, videoID)
		if err != nil {
			return nil, err
		}
	}

	if deltaFavorite != 0 {
		if authorID, authorErr := s.authors.AuthorID(ctx, videoID); authorErr == nil {
			_ = s.creators.IncrFavorites(ctx, authorID, deltaFavorite)
		}
	}

	s.folderC.Invalidate(userID)

	return &FavoriteResult{FavoriteCount: stats.FavoriteCount}, nil
}

func (s *InteractionService) addFavorite(
	ctx context.Context,
	videoID, userID, folderID uint,
) (*FavoriteResult, error) {
	authorID := uint(0)
	firstFavorite := false

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if err := s.ledger.LockAccounts(txCtx, []uint{userID}); err != nil {
			return err
		}

		existsAny, err := s.folders.UserFavorited(txCtx, videoID, userID)
		if err != nil {
			return err
		}

		created, err := s.folders.EnsureLink(txCtx, folderID, videoID)
		if err != nil {
			return err
		}

		if !created || existsAny {
			return nil
		}

		firstFavorite = true

		authorID, err = s.videos.PublishedAuthorIDByVideoID(txCtx, videoID)
		if err != nil {
			return err
		}

		if authorID == 0 {
			return nil
		}

		return s.ledger.ApplyChange(
			txCtx,
			authorID,
			domain.FavoriteAuthorReward,
			"作品被收藏 +"+coinutil.Format(domain.FavoriteAuthorReward),
			nil,
		)
	}); err != nil {
		return nil, err
	}

	if firstFavorite {
		stats, err := s.stats.ApplyDelta(ctx, videoID, domain.StatsDelta{FavoriteCount: 1})
		if err != nil {
			return nil, err
		}

		s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreFavoriteWeight, 1)

		if authorID > 0 {
			_ = s.creators.IncrFavorites(ctx, authorID, 1)
		}

		s.folderC.Invalidate(userID)

		return &FavoriteResult{FavoriteCount: stats.FavoriteCount}, nil
	}

	stats, err := s.stats.Load(ctx, videoID)
	if err != nil {
		return nil, err
	}

	s.folderC.Invalidate(userID)

	return &FavoriteResult{FavoriteCount: stats.FavoriteCount}, nil
}

// HasCoined 判断用户是否已对该视频投币。
func (s *InteractionService) HasCoined(ctx context.Context, videoID, userID uint) bool {
	ok, err := s.coins.Exists(orTODO(ctx), videoID, userID)

	return err == nil && ok
}
