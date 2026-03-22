package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// ExpService 经验用例服务。
type ExpService struct {
	txm     tx.Manager
	users   identitydomain.UserRepository
	records identitydomain.ExpRecordRepository
}

// NewExpService 构造经验用例服务。
func NewExpService(txm tx.Manager, users identitydomain.UserRepository, records identitydomain.ExpRecordRepository) *ExpService {
	return &ExpService{txm: txm, users: users, records: records}
}

// LoginExp 每日登录加经验（同一用户每天最多一次）.
func (s *ExpService) LoginExp(ctx context.Context, userID uint) error {
	if userID == 0 {
		return identitydomain.ErrUserNotLogin
	}

	key := "login:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, identitydomain.ExpPerLogin, fmt.Sprintf("每日登录 +%d", identitydomain.ExpPerLogin), &key)

	return err
}

// LikeExp 点赞加经验（同一用户每天最多一次，防循环点赞刷经验）.
func (s *ExpService) LikeExp(ctx context.Context, userID uint) error {
	if userID == 0 {
		return identitydomain.ErrUserNotLogin
	}

	key := "like:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, identitydomain.ExpPerLike, fmt.Sprintf("点赞 +%d", identitydomain.ExpPerLike), &key)

	return err
}

// CoinExpForVideo 投币加经验（同一用户同一视频仅一次，与投币关系对齐）.
func (s *ExpService) CoinExpForVideo(ctx context.Context, userID, videoID uint, coins int64) error {
	if userID == 0 {
		return identitydomain.ErrUserNotLogin
	}

	if videoID == 0 {
		return s.applyCoinExp(ctx, userID, coins, nil)
	}

	key := fmt.Sprintf("coin:%d", videoID)

	return s.applyCoinExp(ctx, userID, coins, &key)
}

// WatchExp 每日观看视频加经验（同一用户每天最多一次）.
func (s *ExpService) WatchExp(ctx context.Context, userID uint) error {
	if userID == 0 {
		return identitydomain.ErrUserNotLogin
	}

	key := "watch:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, identitydomain.ExpPerWatch, fmt.Sprintf("观看视频 +%d", identitydomain.ExpPerWatch), &key)

	return err
}

func (s *ExpService) applyCoinExp(ctx context.Context, userID uint, coins int64, dedupKey *string) error {
	delta, eligible := (identitydomain.ExperiencePolicy{}).CoinReward(coins)
	if !eligible {
		return nil
	}

	reason := fmt.Sprintf("投币 x%d +%d", coins, delta)
	_, _, err := s.addExpWithRecord(ctx, userID, delta, reason, dedupKey)

	return err
}

func (s *ExpService) addExpWithRecord(
	ctx context.Context,
	userID uint,
	delta int64,
	reason string,
	dedupKey *string,
) (*identitydomain.User, bool, error) {
	if userID == 0 {
		return nil, false, identitydomain.ErrUserNotLogin
	}

	if delta == 0 {
		return nil, false, nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	if reason == "" {
		reason = "经验变动"
	}

	var out identitydomain.User

	applied := false

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		account, err := s.users.ExpAndLevelForUpdate(txCtx, userID)
		if err != nil {
			return err
		}

		var gainedToday int64
		if delta > 0 {
			gainedToday, err = s.records.SumDailyGained(txCtx, userID, time.Now())
			if err != nil {
				return err
			}
		}
		applyDelta, allowed := account.ApplyExperience(delta, gainedToday)
		if !allowed {
			return nil
		}

		recReason := reason
		if applyDelta > 0 && applyDelta != delta {
			recReason = fmt.Sprintf("%s（日上限截断 +%d→+%d）", reason, delta, applyDelta)
		}

		if createErr := s.records.Create(txCtx, userID, applyDelta, recReason, dedupKey); createErr != nil {
			if errors.Is(createErr, identitydomain.ErrDuplicateRecord) {
				return nil
			}

			return createErr
		}

		if updateErr := s.users.UpdateExpAndLevel(txCtx, userID, account.Exp(), account.Level()); updateErr != nil {
			return updateErr
		}

		out = *account
		applied = true

		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if !applied {
		return nil, false, nil
	}

	return &out, true, nil
}
