package social

import (
	"context"

	socialdomain "f_ccg_video_back_ddd/internal/domain/social"
)

// RelationService 关注关系读侧用例服务。
//
// 所有查询以 context 作为首参，向下游传递请求取消信号。
type RelationService struct {
	query   RelationQuery
	privacy socialdomain.PrivacyPolicy
}

// NewRelationService 构造关注关系查询服务。
func NewRelationService(query RelationQuery) *RelationService {
	return &RelationService{query: query, privacy: socialdomain.PrivacyPolicy{}}
}

// FocusCount 获取用户的关注数。
func (s *RelationService) FocusCount(ctx context.Context, userID uint) int64 {
	return s.query.Counts(ctx, userID, false).FocusCount
}

// FansCount 获取用户的粉丝数。
func (s *RelationService) FansCount(ctx context.Context, userID uint) int64 {
	return s.query.Counts(ctx, userID, false).FansCount
}

// IsFocus 判断 userID 是否关注了 targetUserID。
func (s *RelationService) IsFocus(ctx context.Context, userID, targetUserID uint) bool {
	return s.query.Exists(ctx, userID, targetUserID)
}

// FocusRelation 获取两个用户之间的关注关系。
func (s *RelationService) FocusRelation(ctx context.Context, userID, targetUserID uint) (isFocus, isFans bool) {
	relation := s.query.Relation(ctx, userID, targetUserID)

	return relation.IsFocus, relation.IsFans
}

// RelationCounts 一次取回关注数/粉丝数。
func (s *RelationService) RelationCounts(
	ctx context.Context,
	userID uint,
	withDynamic bool,
) socialdomain.RelationCounts {
	return s.query.Counts(ctx, userID, withDynamic)
}

// FollowPrivacy 读取关注/粉丝列表公开性。
func (s *RelationService) FollowPrivacy(ctx context.Context, userID uint) (socialdomain.FollowPrivacy, error) {
	if err := s.privacy.EnsureUserID(userID); err != nil {
		return socialdomain.FollowPrivacy{}, err
	}

	return s.query.FollowPrivacy(ctx, userID)
}

// ActionPrivacy 读取最近点赞/投币公开性。
func (s *RelationService) ActionPrivacy(ctx context.Context, userID uint) (socialdomain.ActionPrivacy, error) {
	if err := s.privacy.EnsureUserID(userID); err != nil {
		return socialdomain.ActionPrivacy{}, err
	}

	return s.query.ActionPrivacy(ctx, userID)
}

// FocusStatus 查询当前用户与目标用户的关注关系（查询自己时返回领域错误）。
func (s *RelationService) FocusStatus(ctx context.Context, viewerID uint, targetID int) (socialdomain.Relation, error) {
	if err := s.privacy.EnsureRelationQueryable(int(viewerID), targetID); err != nil {
		return socialdomain.Relation{}, err
	}

	return s.query.Relation(ctx, viewerID, uint(targetID)), nil
}

// FocusList 关注列表用例：隐私与翻页校验 + 分页查询。
func (s *RelationService) FocusList(
	ctx context.Context,
	params SocialListParams,
) ([]SocialListUser, int64, error) {
	privacy, err := s.FollowPrivacy(ctx, uint(params.OwnerID))
	if err != nil {
		// 读取隐私配置失败一律按「用户不存在」对外呈现。
		return nil, 0, socialdomain.ErrUserNotFound
	}

	if err := s.privacy.EnsureFollowListVisible(privacy, params.ViewerID, params.OwnerID, params.Page); err != nil {
		return nil, 0, err
	}

	rows, total, err := s.query.ListFocus(ctx, uint(params.OwnerID), params.Page, params.PageSize, params.Keyword)
	if err != nil {
		return nil, 0, socialdomain.ErrListQueryFailed
	}

	return rows, total, nil
}

// FansList 粉丝列表用例：隐私与翻页校验 + 分页查询。
func (s *RelationService) FansList(
	ctx context.Context,
	params SocialListParams,
) ([]SocialListUser, int64, error) {
	privacy, err := s.FollowPrivacy(ctx, uint(params.OwnerID))
	if err != nil {
		return nil, 0, socialdomain.ErrUserNotFound
	}

	if err := s.privacy.EnsureFansListVisible(privacy, params.ViewerID, params.OwnerID, params.Page); err != nil {
		return nil, 0, err
	}

	rows, total, err := s.query.ListFans(ctx, uint(params.OwnerID), params.Page, params.PageSize, params.Keyword)
	if err != nil {
		return nil, 0, socialdomain.ErrListQueryFailed
	}

	return rows, total, nil
}
