package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	identityerrors "f_ccg_video_back_ddd/internal/domain/identity/domainerr"
	identityentity "f_ccg_video_back_ddd/internal/domain/identity/entity"
	identityrepo "f_ccg_video_back_ddd/internal/domain/identity/repository"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/coinutil"
)

// CoinService 硬币用例服务。
//
// tx.Manager 开启事务，context 向仓储传递事务句柄，以便硬币流水与调用方写入共享同一事务。
type CoinService struct {
	txm      tx.Manager
	accounts identityrepo.CoinAccountRepository
	records  identityrepo.CoinRecordRepository
}

// NewCoinService 构造硬币用例服务。
func NewCoinService(txm tx.Manager, accounts identityrepo.CoinAccountRepository, records identityrepo.CoinRecordRepository) *CoinService {
	return &CoinService{txm: txm, accounts: accounts, records: records}
}

// LockAccounts 为跨账户的投币/奖励操作预先取得统一顺序的锁。
// 必须在调用方的事务内调用；锁的实现与账户数据均由 identity 管理。
func (s *CoinService) LockAccounts(ctx context.Context, userIDs []uint) error {
	return s.accounts.LockAccounts(ctx, userIDs)
}

// CoinChangeResult 描述本次记账结果；Applied=false 表示零变动或流水已存在。
type CoinChangeResult struct {
	UserID  uint
	Balance float64
	Applied bool
}

// ApplyChange 在账户行锁内执行领域行为，余额与流水原子提交。
func (s *CoinService) ApplyChange(ctx context.Context, userID uint, delta float64, reason string, dedupKey *string) (CoinChangeResult, error) {
	result := CoinChangeResult{UserID: userID}
	if ctx == nil {
		ctx = context.TODO()
	}
	if userID == 0 {
		return result, identityerrors.ErrUserNotLogin
	}
	delta = coinutil.Round(delta)
	if delta == 0 {
		return result, nil
	}
	if reason == "" {
		reason = "硬币变动"
	}
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		account, err := s.accounts.FindForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		result.Balance = account.Balance()
		if dedupKey != nil {
			exists, err := s.records.HasDedupKey(txCtx, userID, *dedupKey)
			if err != nil || exists {
				return err
			}
		}
		if err := account.ApplyChange(delta); err != nil {
			return err
		}
		if err := s.records.Create(txCtx, userID, delta, reason, dedupKey); err != nil {
			if errors.Is(err, identityerrors.ErrDuplicateRecord) {
				return nil
			}
			return err
		}
		if err := s.accounts.SaveBalance(txCtx, account); err != nil {
			return err
		}
		result.Balance = account.Balance()
		result.Applied = true
		return nil
	})
	if err != nil {
		return CoinChangeResult{UserID: userID}, err
	}
	return result, nil
}

// LoginCoin 每日登录加硬币（同一用户每天最多一次）。
func (s *CoinService) LoginCoin(ctx context.Context, userID uint) error {
	if userID == 0 {
		return identityerrors.ErrUserNotLogin
	}

	key := "login:" + time.Now().Format("20060102")
	_, err := s.ApplyChange(
		ctx,
		userID, identityentity.CoinLoginReward, "每日登录 +"+coinutil.Format(identityentity.CoinLoginReward),
		&key,
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
		if err := s.accounts.LockAccounts(txCtx, []uint{authorID}); err != nil {
			return err
		}
		key := fmt.Sprintf("favorite:%d:%d", actorID, videoID)
		earned, err := s.records.HasDedupKey(txCtx, authorID, key)
		if err != nil || !earned {
			return err
		}
		account, err := s.accounts.FindForUpdate(txCtx, authorID)
		if err != nil {
			return err
		}
		revokeKey := "revoke:" + key
		if err := s.records.Create(txCtx, authorID, -identityentity.CoinFavoriteReward, "取消收藏，收回作品收藏奖励", &revokeKey); err != nil {
			if errors.Is(err, identityerrors.ErrDuplicateRecord) {
				return nil
			}
			return err
		}
		if err := account.RevokeFavoriteReward(); err != nil {
			return err
		}
		return s.accounts.SaveBalance(txCtx, account)
	})
}
