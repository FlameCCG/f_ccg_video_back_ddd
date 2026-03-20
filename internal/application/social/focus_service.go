package social

import (
	"context"
	"errors"

	socialerrors "f_ccg_video_back_ddd/internal/domain/social/domainerr"
	socialentity "f_ccg_video_back_ddd/internal/domain/social/entity"
	socialrepo "f_ccg_video_back_ddd/internal/domain/social/repository"

	"go.uber.org/zap"
)

// FocusService 关注/取消关注用例服务。
type FocusService struct {
	repo    socialrepo.FocusRepository
	users   UserExistsQuery
	metrics CreatorFansMetric
	logger  *zap.Logger
}

// NewFocusService 构造关注用例服务。
func NewFocusService(
	repo socialrepo.FocusRepository,
	users UserExistsQuery,
	metrics CreatorFansMetric,
	_ UserSearchSyncer,
	logger *zap.Logger,
) *FocusService {
	return &FocusService{repo: repo, users: users, metrics: metrics, logger: logger}
}

// Focus 关注目标用户。
//
// 先拒绝关注自己，再校验目标用户并写入关注关系；唯一键冲突视为已关注，成功后粉丝数指标 +1。
// user_focus 的搜索索引由 River CDC 异步更新，写请求不依赖搜索上下文。
func (s *FocusService) Focus(ctx context.Context, userID, focusUserID uint) error {
	focus, err := socialentity.NewFocus(userID, focusUserID)
	if err != nil {
		return err
	}

	// 存在性查询报错也按「目标用户不存在」对外呈现。
	exists, err := s.users.Exists(ctx, focusUserID)
	if err != nil || !exists {
		return socialerrors.ErrTargetUserNotFound
	}

	if err := s.repo.Create(ctx, focus); err != nil {
		if errors.Is(err, socialerrors.ErrAlreadyFocused) {
			return socialerrors.ErrAlreadyFocused
		}

		s.logger.Error("关注用户失败", zap.Error(err))

		return socialerrors.ErrFocusFailed
	}

	s.updateFansMetric(ctx, focusUserID, 1)

	return nil
}

// Unfocus 取消关注目标用户。
//
// 流程删除关注关系 → 无匹配记录时返回「未关注该用户」
// → 粉丝数指标 -1；搜索索引由 River CDC 异步更新。
func (s *FocusService) Unfocus(ctx context.Context, userID, focusUserID uint) error {
	affected, err := s.repo.Delete(ctx, userID, focusUserID)
	if err != nil {
		s.logger.Error("取消关注失败", zap.Error(err))

		return socialerrors.ErrUnfocusFailed
	}

	if affected == 0 {
		return socialerrors.ErrNotFocused
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
