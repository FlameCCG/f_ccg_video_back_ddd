package social

import (
	"context"
	"f_ccg_video_back_ddd/pkg/apperr"

	domain "f_ccg_video_back_ddd/internal/domain/social"
)

// RelationService 关注关系读侧用例服务（原 service/user_service/focus_service.go）。
//
// 原实现的 GetXxx 方法有的不接 context（内部用 context.TODO()），
// 新架构统一以 context 作为首参，请求取消可以向下传递。
type RelationService struct {
	query RelationQuery
}

// NewRelationService 构造关注关系查询服务。
func NewRelationService(query RelationQuery) *RelationService {
	return &RelationService{query: query}
}

// FocusCount 获取用户的关注数（原 GetFocusCount）。
func (s *RelationService) FocusCount(ctx context.Context, userID uint) int64 {
	return s.query.Counts(ctx, userID, false).FocusCount
}

// FansCount 获取用户的粉丝数（原 GetFansCount）。
func (s *RelationService) FansCount(ctx context.Context, userID uint) int64 {
	return s.query.Counts(ctx, userID, false).FansCount
}

// IsFocus 判断 userID 是否关注了 targetUserID（原 IsFocus）。
func (s *RelationService) IsFocus(ctx context.Context, userID, targetUserID uint) bool {
	return s.query.Exists(ctx, userID, targetUserID)
}

// FocusRelation 获取两个用户之间的关注关系（原 GetFocusRelation）。
func (s *RelationService) FocusRelation(ctx context.Context, userID, targetUserID uint) (isFocus, isFans bool) {
	relation := s.query.Relation(ctx, userID, targetUserID)

	return relation.IsFocus, relation.IsFans
}

// RelationCounts 一次取回关注数/粉丝数（withDynamic 为真时附带动态数，原 GetRelationCounts）。
func (s *RelationService) RelationCounts(
	ctx context.Context,
	userID uint,
	withDynamic bool,
) domain.RelationCounts {
	return s.query.Counts(ctx, userID, withDynamic)
}

// FollowPrivacy 读取关注/粉丝列表公开性（原 GetFollowPrivacy）。
func (s *RelationService) FollowPrivacy(ctx context.Context, userID uint) (domain.FollowPrivacy, error) {
	if userID == 0 {
		return domain.FollowPrivacy{}, domain.ErrUserIDRequired
	}

	return s.query.FollowPrivacy(ctx, userID)
}

// ActionPrivacy 读取最近点赞/投币公开性（原 GetActionPrivacy）。
func (s *RelationService) ActionPrivacy(ctx context.Context, userID uint) (domain.ActionPrivacy, error) {
	if userID == 0 {
		return domain.ActionPrivacy{}, domain.ErrUserIDRequired
	}

	return s.query.ActionPrivacy(ctx, userID)
}

// FocusStatus 查询当前用户与目标用户的关注关系（查询自己时返回领域错误）。
func (s *RelationService) FocusStatus(ctx context.Context, viewerID uint, targetID int) (domain.Relation, error) {
	if err := domain.EnsureRelationQueryable(int(viewerID), targetID); err != nil {
		return domain.Relation{}, err
	}

	return s.query.Relation(ctx, viewerID, uint(targetID)), nil
}

// FocusList 关注列表用例：隐私与翻页校验 + 分页查询（原 ListFocusUsers 的调用流程）。
func (s *RelationService) FocusList(ctx context.Context, params ListParams) ([]ListUser, int64, error) {
	return s.visibleRelationList(ctx, params, domain.EnsureFollowListVisible, s.query.ListFocus)
}

// FansList 粉丝列表用例：隐私与翻页校验 + 分页查询（原 ListFansUsers 的调用流程）。
func (s *RelationService) FansList(ctx context.Context, params ListParams) ([]ListUser, int64, error) {
	return s.visibleRelationList(ctx, params, domain.EnsureFansListVisible, s.query.ListFans)
}

func (s *RelationService) visibleRelationList(ctx context.Context, params ListParams, visible func(domain.FollowPrivacy, int, int, int) error, query func(context.Context, uint, int, int, string) ([]ListUser, int64, error)) ([]ListUser, int64, error) {
	privacy, err := s.FollowPrivacy(ctx, uint(params.OwnerID))
	if err != nil {
		return nil, 0, apperr.Wrap("application/social/relation_service", domain.ErrUserNotFound, err)
	}

	if err := visible(privacy, params.ViewerID, params.OwnerID, params.Page); err != nil {
		return nil, 0, err
	}

	rows, total, err := query(ctx, uint(params.OwnerID), params.Page, params.PageSize, params.Keyword)
	if err != nil {
		return nil, 0, apperr.Wrap("application/social/relation_service", domain.ErrListQueryFailed, err)
	}

	return rows, total, nil
}
