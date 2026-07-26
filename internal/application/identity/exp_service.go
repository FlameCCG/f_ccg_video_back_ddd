package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// ExpService 经验用例服务（原 service/user_service/exp_service.go）。
type ExpService struct {
	txm     tx.Manager
	users   domain.UserRepository
	records domain.ExpRecordRepository
}

// NewExpService 构造经验用例服务。
func NewExpService(txm tx.Manager, users domain.UserRepository, records domain.ExpRecordRepository) *ExpService {
	return &ExpService{txm: txm, users: users, records: records}
}

// AddExp 增加经验并返回最新等级/经验（reason 用于记录原因）.
func (s *ExpService) AddExp(ctx context.Context, userID uint, delta int64, reason string) (*domain.User, error) {
	user, _, err := s.addExpWithRecord(ctx, userID, delta, reason, nil)

	return user, err
}

// LoginExp 每日登录加经验（同一用户每天最多一次）.
func (s *ExpService) LoginExp(ctx context.Context, userID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	key := "login:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, domain.ExpPerLogin, fmt.Sprintf("每日登录 +%d", domain.ExpPerLogin), &key)

	return err
}

// LikeExp 点赞加经验（同一用户每天最多一次，防循环点赞刷经验）.
func (s *ExpService) LikeExp(ctx context.Context, userID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	key := "like:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, domain.ExpPerLike, fmt.Sprintf("点赞 +%d", domain.ExpPerLike), &key)

	return err
}

// CoinExp 投币加经验（无视频维度幂等；优先用 CoinExpForVideo）.
func (s *ExpService) CoinExp(ctx context.Context, userID uint, coins int64) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	return s.applyCoinExp(ctx, userID, coins, nil)
}

// CoinExpForVideo 投币加经验（同一用户同一视频仅一次，与投币关系对齐）.
func (s *ExpService) CoinExpForVideo(ctx context.Context, userID, videoID uint, coins int64) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
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
		return domain.ErrUserNotLogin
	}

	key := "watch:" + time.Now().Format("20060102")
	_, _, err := s.addExpWithRecord(ctx, userID, domain.ExpPerWatch, fmt.Sprintf("观看视频 +%d", domain.ExpPerWatch), &key)

	return err
}

func (s *ExpService) applyCoinExp(ctx context.Context, userID uint, coins int64, dedupKey *string) error {
	if coins <= 0 {
		return nil
	}

	delta := coins * domain.ExpPerCoin
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
) (*domain.User, bool, error) {
	if userID == 0 {
		return nil, false, domain.ErrUserNotLogin
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

	var out domain.User

	applied := false

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		beforeExp, _, err := s.users.ExpAndLevelForUpdate(txCtx, userID)
		if err != nil {
			return err
		}

		// 正向加经验受每日总上限约束；负向调整不占用日额度。
		applyDelta := delta

		if applyDelta > 0 {
			gained, sumErr := s.records.SumDailyGained(txCtx, userID, time.Now())
			if sumErr != nil {
				return sumErr
			}

			clamped, allowed := domain.ClampDailyExpGain(applyDelta, gained)
			if !allowed {
				return nil
			}

			applyDelta = clamped
		}

		afterExp := max(beforeExp+applyDelta, 0)
		afterLevel := domain.CalcLevel(afterExp)

		recReason := reason
		if applyDelta > 0 && applyDelta != delta {
			recReason = fmt.Sprintf("%s（日上限截断 +%d→+%d）", reason, delta, applyDelta)
		}

		if createErr := s.records.Create(txCtx, userID, applyDelta, recReason, dedupKey); createErr != nil {
			if errors.Is(createErr, domain.ErrDuplicateRecord) {
				return nil
			}

			return createErr
		}

		if updateErr := s.users.UpdateExpAndLevel(txCtx, userID, afterExp, afterLevel); updateErr != nil {
			return updateErr
		}

		out = domain.User{ID: userID, Exp: afterExp, Level: afterLevel}
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
