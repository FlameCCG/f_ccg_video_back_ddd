package social

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"

	domain "f_ccg_video_back_ddd/internal/domain/social"

	"go.uber.org/zap"
)

// FocusService 关注/取消关注用例服务（原 api/social_api/focus_user.go、unfocus_user.go
// 中的写侧流程，从 interfaces 层下沉到应用层）。
type FocusService struct {
	repo    domain.FocusRepository
	users   UserExistsQuery
	metrics CreatorFansMetric
	logger  *zap.Logger
}

// NewFocusService 构造关注用例服务。
func NewFocusService(
	repo domain.FocusRepository,
	users UserExistsQuery,
	metrics CreatorFansMetric,
	_ UserSearchSyncer,
	logger *zap.Logger,
) *FocusService {
	return &FocusService{repo: repo, users: users, metrics: metrics, logger: logger}
}

// Focus 关注目标用户。
//
// 流程与原实现一致：禁止关注自己 → 校验目标用户存在 → 写入关注关系
// （唯一键冲突视为「已关注」）→ 粉丝数指标 +1。
// user_focus 的搜索索引更新由 River CDC 异步消费，写请求不再同步依赖搜索上下文。
func (s *FocusService) Focus(ctx context.Context, userID, focusUserID uint) error {
	focus, err := domain.NewFocus(userID, focusUserID)
	if err != nil {
		return err
	}

	// 与原实现一致：存在性查询报错也按「目标用户不存在」对外呈现。
	exists, err := s.users.Exists(ctx, focusUserID)
	if err != nil || !exists {
		return apperr.Wrap("application/social/focus_service", domain.ErrTargetUserNotFound, err)
	}

	if err := s.repo.Create(ctx, focus); err != nil {
		if errors.Is(err, domain.ErrAlreadyFocused) {
			return domain.ErrAlreadyFocused
		}

		return apperr.Wrap("application/social/focus_service", domain.ErrFocusFailed, err)
	}

	s.updateFansMetric(ctx, focusUserID, 1)

	return nil
}

// Unfocus 取消关注目标用户。
//
// 流程与原实现一致：删除关注关系 → 无匹配记录时返回「未关注该用户」
// → 粉丝数指标 -1；搜索索引由 River CDC 异步更新。
func (s *FocusService) Unfocus(ctx context.Context, userID, focusUserID uint) error {
	affected, err := s.repo.Delete(ctx, userID, focusUserID)
	if err != nil {
		return apperr.Wrap("application/social/focus_service", domain.ErrUnfocusFailed, err)
	}

	if affected == 0 {
		return domain.ErrNotFocused
	}

	s.updateFansMetric(ctx, focusUserID, -1)

	return nil
}

// updateFansMetric 关注关系变更后累加粉丝指标；失败不影响关系写入结果。
func (s *FocusService) updateFansMetric(ctx context.Context, focusUserID uint, delta int64) {
	if s.metrics != nil {
		_ = s.metrics.IncrFans(ctx, focusUserID, delta)
	}
}
