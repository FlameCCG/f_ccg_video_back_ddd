package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/coinutil"
)

// CoinService 硬币用例服务（原 service/user_service/coin_service.go）。
//
// 原 ApplyChangeTx(tx *gorm.DB, ...) 供其它模块（视频三连）加入调用方事务；
// 新架构由 tx.Manager + context 传递事务实现同一语义：
// 在已有事务的 context 中调用 ApplyChange 会直接复用该事务，不开保存点。
type CoinService struct {
	txm     tx.Manager
	users   domain.UserRepository
	records domain.CoinRecordRepository
}

// NewCoinService 构造硬币用例服务。
func NewCoinService(txm tx.Manager, users domain.UserRepository, records domain.CoinRecordRepository) *CoinService {
	return &CoinService{txm: txm, users: users, records: records}
}

// LockAccounts 为跨账户的投币/奖励操作预先取得统一顺序的锁。
// 必须在调用方的事务内调用；锁的实现与账户数据均由 identity 管理。
func (s *CoinService) LockAccounts(ctx context.Context, userIDs []uint) error {
	return s.users.LockAccounts(ctx, userIDs)
}

// ApplyChange 增减用户硬币余额并写入记录。
//
// 余额在事务行锁内读取，流水创建与余额更新共同提交；dedupKey 通过记录唯一性阻止重复发奖。
// 返回的 bool 表示是否实际应用；去重命中可返回 (nil, false, nil)，不能把 nil 用户当成“用户不存在”。
func (s *CoinService) ApplyChange(
	ctx context.Context,
	userID uint,
	delta float64,
	reason string,
	dedupKey *string,
) (*domain.User, bool, error) {
	if ctx == nil {
		ctx = context.TODO()
	}

	if userID == 0 {
		return nil, false, domain.ErrUserNotLogin
	}

	delta = coinutil.Round(delta)
	if delta == 0 {
		return nil, false, nil
	}

	if reason == "" {
		reason = "硬币变动"
	}

	var out *domain.User

	applied := false

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		before, err := s.users.CoinBalanceForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		if dedupKey != nil {
			exists, err := s.records.HasDedupKey(txCtx, userID, *dedupKey)
			if err != nil || exists {
				return err
			}
		}

		afterCoins := coinutil.Round(before + delta)
		if delta < 0 && afterCoins < 0 {
			return domain.ErrCoinInsufficient
		}

		if createErr := s.records.Create(txCtx, userID, delta, reason, dedupKey); createErr != nil {
			if errors.Is(createErr, domain.ErrDuplicateRecord) {
				return nil
			}

			return createErr
		}

		if updateErr := s.users.UpdateCoinBalance(txCtx, userID, afterCoins); updateErr != nil {
			return updateErr
		}

		out = &domain.User{ID: userID, CoinCount: afterCoins}
		applied = true

		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if !applied {
		return nil, false, nil
	}

	return out, true, nil
}

// EnsureEnough 检查用户当前硬币是否足以支付本次消耗。
//
// 这是读取时刻的预检查，之后并发消费仍会改变余额；实际扣减必须由 ApplyChange 的锁内校验裁定。
func (s *CoinService) EnsureEnough(ctx context.Context, userID uint, need float64) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	need = coinutil.Round(need)
	if need <= 0 {
		return nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	balance, err := s.users.CoinBalance(ctx, userID)
	if err != nil {
		return err
	}

	if coinutil.Round(balance) < need {
		return domain.ErrCoinInsufficient
	}

	return nil
}

// LoginCoin 每日登录加硬币（同一用户每天最多一次）。
func (s *CoinService) LoginCoin(ctx context.Context, userID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	key := "login:" + time.Now().Format("20060102")
	_, _, err := s.ApplyChange(
		ctx,
		userID,
		domain.CoinLoginReward,
		"每日登录 +"+coinutil.Format(domain.CoinLoginReward),
		&key,
	)

	return err
}

// FavoriteReward 作品被收藏时给作者增加硬币。
func (s *CoinService) FavoriteReward(ctx context.Context, userID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	_, _, err := s.ApplyChange(
		ctx,
		userID,
		domain.CoinFavoriteReward,
		"作品被收藏 +"+coinutil.Format(domain.CoinFavoriteReward),
		nil,
	)

	return err
}

// CoinReward 作品被投币时给作者增加硬币，奖励为投币数的 10%。
func (s *CoinService) CoinReward(ctx context.Context, userID uint, coins int64) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if coins <= 0 {
		return nil
	}

	delta := coinutil.Round(float64(coins) * domain.CoinRewardRateOfCoined)
	_, _, err := s.ApplyChange(
		ctx,
		userID,
		delta,
		fmt.Sprintf("作品被投币 x%d +%s", coins, coinutil.Format(delta)),
		nil,
	)

	return err
}

// RevokeFavoriteReward reverses the one lifetime reward only if it was granted.
// A reversal can make the earned balance negative if the reward was already spent;
// spending remains disallowed by ApplyChange until the debt is covered.
func (s *CoinService) RevokeFavoriteReward(ctx context.Context, authorID, actorID, videoID uint) error {
	if authorID == 0 || authorID == actorID {
		return nil
	}
	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if err := s.users.LockAccounts(txCtx, []uint{authorID}); err != nil {
			return err
		}
		key := fmt.Sprintf("favorite:%d:%d", actorID, videoID)
		earned, err := s.records.HasDedupKey(txCtx, authorID, key)
		if err != nil || !earned {
			return err
		}
		balance, err := s.users.CoinBalanceForUpdate(txCtx, authorID)
		if err != nil {
			return err
		}
		revokeKey := "revoke:" + key
		if err := s.records.Create(txCtx, authorID, -domain.CoinFavoriteReward, "取消收藏，收回作品收藏奖励", &revokeKey); err != nil {
			if errors.Is(err, domain.ErrDuplicateRecord) {
				return nil
			}
			return err
		}
		return s.users.UpdateCoinBalance(txCtx, authorID, coinutil.Round(balance-domain.CoinFavoriteReward))
	})
}
