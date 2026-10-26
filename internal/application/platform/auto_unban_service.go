package platform

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/pkg/logctx"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/platform"

	"go.uber.org/zap"
)

// autoUnbanTimeout 单轮自动解封的执行窗口（与原实现的 10s 一致）。
const autoUnbanTimeout = 10 * time.Second

// AutoUnbanService 到期用户自动解封用例（原 service/cron_service/auto_unban_user.go）。
type AutoUnbanService struct {
	txm      tx.Manager
	bans     BanMaintenanceRepository
	statuses UserStatusWriter
	logger   *zap.Logger
}

// NewAutoUnbanService 构造自动解封用例服务。
func NewAutoUnbanService(
	txm tx.Manager,
	bans BanMaintenanceRepository,
	statuses UserStatusWriter,
	logger *zap.Logger,
) *AutoUnbanService {
	return &AutoUnbanService{txm: txm, bans: bans, statuses: statuses, logger: logger}
}

// Run 执行一轮自动解封：查出到期封禁 → 事务内批量置正常并标记原记录 → 写穿状态缓存。
//
// 定时任务是调度 root 场景，没有请求上下文，因此内部自带 10s 执行窗口。
// 任何一步失败只记录日志、不向调度器返回错误（与原实现一致）。
func (s *AutoUnbanService) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, autoUnbanTimeout)
	defer cancel()

	now := time.Now()

	expired, err := s.bans.ListExpiredBans(ctx, now)
	if err != nil {
		logctx.With(ctx, s.logger).Error("auto unban query failed", zap.String("event", "platform.auto_unban_service.run.failed"), zap.String("operation", "platform.auto_unban_service.run"), zap.Error(err))

		return
	}

	if len(expired) == 0 {
		return
	}

	plan := domain.PlanAutoUnban(expired)
	if plan.Empty() {
		return
	}

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if updateErr := s.bans.ActivateBannedUsers(txCtx, plan.UserIDs); updateErr != nil {
			return updateErr
		}

		return s.bans.MarkBanRecordsUnbanned(txCtx, plan.RecordIDs)
	}); err != nil {
		logctx.With(ctx, s.logger).Error("auto unban failed", zap.String("event", "platform.auto_unban_service.run.failed"), zap.String("operation", "platform.auto_unban_service.run"), zap.Error(err))

		return
	}

	if s.statuses != nil {
		for _, userID := range plan.UserIDs {
			// P1-3: 解封后 write-through Active，避免仅 DEL 被旧 Banned 回填。
			s.statuses.Write(userID, enum.UserStatusActive)
		}
	}

	logctx.With(ctx, s.logger).Info("auto unban ok", zap.String("event", "platform.auto_unban_service.run.diagnostic"), zap.String("operation", "platform.auto_unban_service.run"), zap.Int("count", len(plan.UserIDs)))
}
