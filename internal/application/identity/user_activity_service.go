package identity

import (
	"context"
	"f_ccg_video_back_ddd/pkg/apperr"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

const (
	// recentActionLimit 最近点赞/投币固定返回条数（与原实现一致）。
	recentActionLimit = 10

	// userSearchDefaultPageSize 用户搜索默认每页条数。
	userSearchDefaultPageSize = 20
	// userSearchMaxPageSize 用户搜索每页条数上限。
	userSearchMaxPageSize = 100
	// mentionDefaultPageSize @ 联想默认每页条数。
	mentionDefaultPageSize = 20
	// mentionMaxPageSize @ 联想每页条数上限。
	mentionMaxPageSize = 50
	// userVideoDefaultPageSize 用户主页视频默认每页条数。
	userVideoDefaultPageSize = 20
)

// UserActivityService 挂在用户路由下、但由其它上下文提供数据的用例服务
// （原 api/user_api/user_videos.go、recent_actions.go、delete_video.go、
// create_tag.go、search_user.go、mention_suggest.go）。
//
// 本服务只做参数归一化与隐私校验，数据一律经跨上下文端口获取。
type UserActivityService struct {
	relations UserRelationQuery
	videos    UserVideoQuery
	tags      VideoTagWriter
	remover   VideoRemover
	search    UserSearchQuery
	mentions  MentionSuggestQuery
}

// NewUserActivityService 构造用户活动服务。
func NewUserActivityService(
	relations UserRelationQuery,
	videos UserVideoQuery,
	tags VideoTagWriter,
	remover VideoRemover,
	search UserSearchQuery,
	mentions MentionSuggestQuery,
) *UserActivityService {
	return &UserActivityService{
		relations: relations,
		videos:    videos,
		tags:      tags,
		remover:   remover,
		search:    search,
		mentions:  mentions,
	}
}

// ListUserVideos 用户主页视频分页列表。
func (s *UserActivityService) ListUserVideos(
	ctx context.Context,
	userID, viewerID uint,
	page, pageSize, sort, auditStatus int,
) (any, int64, error) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = userVideoDefaultPageSize
	}

	return s.videos.ListUserVideos(ctx, userID, viewerID, page, pageSize, sort, auditStatus)
}

// RecentLiked 最近点赞的视频（受 openLikeVideo 隐私开关约束）。
func (s *UserActivityService) RecentLiked(ctx context.Context, targetID, viewerID uint) (any, int64, error) {
	if err := s.ensureActionVisible(ctx, targetID, viewerID, VideoActionLike); err != nil {
		return nil, 0, err
	}

	return s.recentActions(ctx, targetID, VideoActionLike)
}

// RecentCoined 最近投币的视频（受 openCoinVideo 隐私开关约束）。
func (s *UserActivityService) RecentCoined(ctx context.Context, targetID, viewerID uint) (any, int64, error) {
	if err := s.ensureActionVisible(ctx, targetID, viewerID, VideoActionCoin); err != nil {
		return nil, 0, err
	}

	return s.recentActions(ctx, targetID, VideoActionCoin)
}

func (s *UserActivityService) recentActions(ctx context.Context, targetID uint, action string) (any, int64, error) {
	items, count, err := s.videos.RecentActions(ctx, targetID, action, recentActionLimit)
	if err != nil {
		return nil, 0, err
	}

	return items, int64(count), nil
}

// ensureActionVisible 校验访问者是否有权查看目标用户的最近动作列表。
func (s *UserActivityService) ensureActionVisible(ctx context.Context, targetID, viewerID uint, action string) error {
	if viewerID == targetID {
		return nil
	}

	privacy, err := s.relations.ActionPrivacy(ctx, targetID)
	if err != nil || !privacy.Exists {
		return apperr.Wrap("application/identity/user_activity_service", domain.ErrUserNotFound, err)
	}

	switch action {
	case VideoActionLike:
		if !privacy.OpenLikeVideo {
			return domain.ErrRecentLikedNotPublic
		}
	case VideoActionCoin:
		if !privacy.OpenCoinVideo {
			return domain.ErrRecentCoinedNotPublic
		}
	}

	return nil
}

// DeleteVideo 用户删除自己的视频（逻辑删除）。
func (s *UserActivityService) DeleteVideo(ctx context.Context, videoID, userID uint) error {
	return s.remover.DeleteVideo(ctx, videoID, userID)
}

// CreateTag 用户创建（或复用）标签。
func (s *UserActivityService) CreateTag(ctx context.Context, name string) (any, error) {
	return s.tags.CreateOrGetTag(ctx, name)
}

// SearchUsers 用户搜索（分页与排序参数在此归一化）。
func (s *UserActivityService) SearchUsers(ctx context.Context, params UserSearchParams) (any, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 {
		params.PageSize = userSearchDefaultPageSize
	}

	if params.PageSize > userSearchMaxPageSize {
		params.PageSize = userSearchMaxPageSize
	}

	params.UserSort = NormalizeUserSort(params.UserSort)
	params.UserOrder = NormalizeSortOrder(params.UserOrder)

	return s.search.SearchUsers(ctx, params)
}

// SuggestMentions 评论 @ 联想（按好友优先返回关注列表）。
func (s *UserActivityService) SuggestMentions(
	ctx context.Context,
	userID uint,
	keyword string,
	page, pageSize int,
) (any, int64, error) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = mentionDefaultPageSize
	}

	if pageSize > mentionMaxPageSize {
		pageSize = mentionMaxPageSize
	}

	return s.mentions.SuggestFollowUsers(ctx, userID, keyword, page, pageSize)
}

// NormalizeUserSort 归一化用户搜索排序字段；未传或非法值默认按粉丝数排序。
func NormalizeUserSort(sort int) int {
	switch sort {
	case UserSortRelevance, UserSortFollowers, UserSortLevel, UserSortCreatedAt:
		return sort
	default:
		// 用户搜索未传或传非法值时，默认按粉丝数降序。
		return UserSortFollowers
	}
}

// NormalizeSortOrder 归一化排序方向；仅显式传 1 时升序，其余按降序处理。
func NormalizeSortOrder(order int) int {
	if order == SortOrderAsc {
		return SortOrderAsc
	}

	return SortOrderDesc
}
