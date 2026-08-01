package contentops

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// StatService 站点流量统计用例服务（原 service/site_service/stat_service.go）。
//
// PV/UV/在线人数的存储细节由 SiteStatStore 端口承载；
// 每日首次打点的登录奖励沿用原实现的 fire-and-forget 语义，
// 由注入的 detached 任务组托管（进程退出时统一取消并等待）。
type StatService struct {
	store    SiteStatStore
	query    AdminStatsQuery
	detached *ctxutil.DetachedTaskGroup
	exp      LoginExpGranter
	coin     LoginCoinGranter
	logger   *zap.Logger
}

// NewStatService 构造站点统计用例服务。
func NewStatService(
	store SiteStatStore,
	query AdminStatsQuery,
	detached *ctxutil.DetachedTaskGroup,
	exp LoginExpGranter,
	coin LoginCoinGranter,
	logger *zap.Logger,
) *StatService {
	return &StatService{
		store:    store,
		query:    query,
		detached: detached,
		exp:      exp,
		coin:     coin,
		logger:   logger,
	}
}

// Touch 站点打点：累加 PV、记录当日 UV、维护在线用户，
// 并在登录用户当日首次打点时异步发放登录经验与硬币。
func (s *StatService) Touch(ctx context.Context, userID uint, ip string) (*TouchResult, error) {
	result, isFirstToday, err := s.store.Touch(ctx, userID, ip)
	if err != nil {
		return nil, err
	}

	// 每日登录奖励：只在当日首次打点时触发（最终幂等由 DB 唯一键兜底）
	if userID > 0 && isFirstToday {
		s.grantDailyLoginReward(ctx, userID)
	}

	return result, nil
}

// Heartbeat 在线心跳：不累加 PV，只维护当日 UV 与在线人数（支持游客）。
func (s *StatService) Heartbeat(ctx context.Context, userID uint, ip string) (*HeartbeatResult, error) {
	return s.store.Heartbeat(ctx, userID, ip)
}

// OnlineCount 读取当前在线人数（同时被管理端统计复用）。
func (s *StatService) OnlineCount(ctx context.Context) (int64, error) {
	return s.store.OnlineCount(ctx)
}

// DailyUV 读取指定自然日的访问 UV（同时被管理端统计复用）。
func (s *StatService) DailyUV(ctx context.Context, day time.Time) (int64, error) {
	return s.store.DailyUV(ctx, day)
}

// grantDailyLoginReward 异步发放每日登录经验与硬币（失败仅告警，不影响打点响应）。
func (s *StatService) grantDailyLoginReward(ctx context.Context, userID uint) {
	if s.detached == nil {
		return
	}

	s.detached.Go(ctx, ctxutil.AsyncDefaultTimeout, func(bg context.Context) {
		if s.exp != nil {
			if err := s.exp.LoginExp(bg, userID); err != nil {
				s.logger.Warn("apply login exp failed", zap.Uint("uid", userID), zap.Error(err))
			}
		}

		if s.coin != nil {
			if err := s.coin.LoginCoin(bg, userID); err != nil {
				s.logger.Warn("apply login coin failed", zap.Uint("uid", userID), zap.Error(err))
			}
		}
	})
}
