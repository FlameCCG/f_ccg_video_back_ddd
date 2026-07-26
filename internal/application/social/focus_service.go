package social

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/social"

	"go.uber.org/zap"
)

// FocusService 关注/取消关注用例服务（原 api/social_api/focus_user.go、unfocus_user.go
// 中的写侧流程，从 interfaces 层下沉到应用层）。
type FocusService struct {
	repo    domain.FocusRepository
	users   UserExistsQuery
	metrics CreatorFansMetric
	search  UserSearchSyncer
}

// NewFocusService 构造关注用例服务。
func NewFocusService(
	repo domain.FocusRepository,
	users UserExistsQuery,
	metrics CreatorFansMetric,
	search UserSearchSyncer,
) *FocusService {
	return &FocusService{repo: repo, users: users, metrics: metrics, search: search}
}

// Focus 关注目标用户。
//
// 流程与原实现一致：禁止关注自己 → 校验目标用户存在 → 写入关注关系
// （唯一键冲突视为「已关注」）→ 粉丝数指标 +1 → 同步双方搜索文档。
// 指标与搜索同步失败只记录日志，不影响关注结果。
func (s *FocusService) Focus(ctx context.Context, userID, focusUserID uint) error {
	focus, err := domain.NewFocus(userID, focusUserID)
	if err != nil {
		return err
	}

	// 与原实现一致：存在性查询报错也按「目标用户不存在」对外呈现。
	exists, err := s.users.Exists(ctx, focusUserID)
	if err != nil || !exists {
		return domain.ErrTargetUserNotFound
	}

	if err := s.repo.Create(ctx, focus); err != nil {
		if errors.Is(err, domain.ErrAlreadyFocused) {
			return domain.ErrAlreadyFocused
		}

		zap.L().Error("关注用户失败", zap.Error(err))

		return domain.ErrFocusFailed
	}

	s.afterRelationChanged(ctx, userID, focusUserID, 1, "关注后同步用户搜索文档失败")

	return nil
}

// Unfocus 取消关注目标用户。
//
// 流程与原实现一致：删除关注关系 → 无匹配记录时返回「未关注该用户」
// → 粉丝数指标 -1 → 同步双方搜索文档。
func (s *FocusService) Unfocus(ctx context.Context, userID, focusUserID uint) error {
	affected, err := s.repo.Delete(ctx, userID, focusUserID)
	if err != nil {
		zap.L().Error("取消关注失败", zap.Error(err))

		return domain.ErrUnfocusFailed
	}

	if affected == 0 {
		return domain.ErrNotFocused
	}

	s.afterRelationChanged(ctx, userID, focusUserID, -1, "取消关注后同步用户搜索文档失败")

	return nil
}

// afterRelationChanged 关注关系变更后的副作用：粉丝数指标累加 + 搜索文档同步。
func (s *FocusService) afterRelationChanged(
	ctx context.Context,
	userID, focusUserID uint,
	delta int64,
	syncFailMsg string,
) {
	if s.metrics != nil {
		_ = s.metrics.IncrFans(ctx, focusUserID, delta)
	}

	if s.search == nil {
		return
	}

	if err := s.search.SyncUserDocuments(ctx, userID, focusUserID); err != nil {
		zap.L().Error(syncFailMsg,
			zap.Uint("userID", userID),
			zap.Uint("focusUserID", focusUserID),
			zap.Error(err))
	}
}
