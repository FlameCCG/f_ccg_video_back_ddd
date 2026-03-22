package identity

import (
	"context"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
)

const (
	// recentActionLimit 最近点赞/投币固定返回条数。
	recentActionLimit = 10
)

// UserActivityService 挂在用户路由下、但由其它上下文提供数据的用例服务。
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
	page, pageSize = (identitydomain.ActivityListingPolicy{}).UserVideoPage(page, pageSize)
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
	if (identitydomain.ActionVisibility{}).IsOwner(targetID, viewerID) {
		return nil
	}

	privacy, err := s.relations.ActionPrivacy(ctx, targetID)
	if err != nil {
		return identitydomain.ErrUserNotFound
	}

	return (identitydomain.ActionVisibility{Exists: privacy.Exists, OpenLikeVideo: privacy.OpenLikeVideo, OpenCoinVideo: privacy.OpenCoinVideo}).EnsureVisible(targetID, viewerID, action)
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
	params.Page, params.PageSize = (identitydomain.ActivityListingPolicy{}).SearchPage(params.Page, params.PageSize)
	params.UserSort = (identitydomain.ActivityListingPolicy{}).UserSort(params.UserSort)
	params.UserOrder = (identitydomain.ActivityListingPolicy{}).SortOrder(params.UserOrder)

	return s.search.SearchUsers(ctx, params)
}

// SuggestMentions 评论 @ 联想（按好友优先返回关注列表）。
func (s *UserActivityService) SuggestMentions(
	ctx context.Context,
	userID uint,
	keyword string,
	page, pageSize int,
) (any, int64, error) {
	page, pageSize = (identitydomain.ActivityListingPolicy{}).MentionPage(page, pageSize)

	return s.mentions.SuggestFollowUsers(ctx, userID, keyword, page, pageSize)
}
