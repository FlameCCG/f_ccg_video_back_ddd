package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/coinutil"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

// CoinRecordService 硬币流水用例服务（原 service/user_service/coin_record_service.go）。
type CoinRecordService struct {
	records domain.CoinRecordRepository
	query   CoinRecordQuery
}

// NewCoinRecordService 构造硬币流水服务。
func NewCoinRecordService(records domain.CoinRecordRepository, query CoinRecordQuery) *CoinRecordService {
	return &CoinRecordService{records: records, query: query}
}

// Record 写入一条硬币流水（不改余额，仅记录）。
func (s *CoinRecordService) Record(ctx context.Context, userID uint, delta float64, reason string) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	delta = coinutil.Round(delta)
	if delta == 0 {
		return nil
	}

	if reason == "" {
		reason = "硬币变动"
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	return s.records.Create(ctx, userID, delta, reason, nil)
}

// List 近 7 天硬币流水分页列表。
func (s *CoinRecordService) List(ctx context.Context, userID uint, page, pageSize int) ([]CoinRecordItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	return s.query.List(ctx, userID, page, pageSize)
}

// ExpRecordService 经验流水用例服务（原 service/user_service/exp_record_service.go）。
type ExpRecordService struct {
	query ExpRecordQuery
}

// NewExpRecordService 构造经验流水服务。
func NewExpRecordService(query ExpRecordQuery) *ExpRecordService {
	return &ExpRecordService{query: query}
}

// List 近 7 天经验流水分页列表。
func (s *ExpRecordService) List(ctx context.Context, userID uint, page, pageSize int) ([]ExpRecordItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	return s.query.List(ctx, userID, page, pageSize)
}

// LoginRecordService 登录记录用例服务（原 service/user_service/login_record_service.go）。
type LoginRecordService struct {
	txm     tx.Manager
	users   domain.UserRepository
	records domain.LoginRecordRepository
	query   LoginRecordQuery
}

// NewLoginRecordService 构造登录记录服务。
func NewLoginRecordService(
	txm tx.Manager,
	users domain.UserRepository,
	records domain.LoginRecordRepository,
	query LoginRecordQuery,
) *LoginRecordService {
	return &LoginRecordService{txm: txm, users: users, records: records, query: query}
}

// Record 更新用户最后登录信息并写入登录记录（同一事务）。
func (s *LoginRecordService) Record(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if ip == "" {
		return domain.ErrIPRequired
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	return s.txm.Do(ctx, func(txCtx context.Context) error {
		if err := s.users.UpdateLastLogin(txCtx, userID, ip, addr); err != nil {
			return err
		}

		return s.records.Create(txCtx, userID, ip, addr, userAgent, loginType)
	})
}

// List 近 7 天登录记录分页列表。
func (s *LoginRecordService) List(ctx context.Context, userID uint, page, pageSize int) ([]LoginRecordItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	return s.query.List(ctx, userID, page, pageSize)
}
