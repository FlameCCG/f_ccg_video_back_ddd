package dynamic

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

// ReadService 「最新作品已读」用例服务（原 service/dynamic_service/read_service.go）。
type ReadService struct {
	reads    domain.ReadRepository
	follows  FollowRelationQuery
	activity ActivityQuery
	users    UserProfileQuery
	feed     *FeedService
	unread   *FollowService
}

// NewReadService 构造已读用例服务。
func NewReadService(
	reads domain.ReadRepository,
	follows FollowRelationQuery,
	activity ActivityQuery,
	users UserProfileQuery,
	feed *FeedService,
	unread *FollowService,
) *ReadService {
	return &ReadService{
		reads:    reads,
		follows:  follows,
		activity: activity,
		users:    users,
		feed:     feed,
		unread:   unread,
	}
}

// ReadLatestAndList 先把作者最新作品标记为已读，再返回该作者的作品流。
func (s *ReadService) ReadLatestAndList(
	ctx context.Context,
	userID, authorID uint,
	page, pageSize int,
) ([]WorkFeedItem, int64, error) {
	if err := s.MarkLatestRead(ctx, userID, authorID); err != nil {
		return nil, 0, err
	}

	return s.feed.ListFeed(ctx, userID, authorID, 0, page, pageSize)
}

// MarkLatestRead 将 author 的「最新作品」标记为已读。
//
// 仅当浏览者是作者本人、或已关注该作者时才写入已读位点（未关注直接返回 nil）。
func (s *ReadService) MarkLatestRead(ctx context.Context, userID, authorID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if authorID == 0 {
		return domain.ErrAuthorIDRequired
	}

	if userID != authorID {
		if !s.follows.IsFollowing(ctx, userID, authorID) {
			return nil
		}
	}

	latest, ok, err := s.latestWork(ctx, authorID)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	return s.upsertRead(ctx, userID, authorID, latest)
}

// MaybeMarkReadByVideo 当用户浏览视频详情/评论相关接口时调用：
// 只有当 (1) 视频作者是关注对象，且 (2) 当前视频就是作者最新作品（视频/动态综合）时才写入已读记录。
func (s *ReadService) MaybeMarkReadByVideo(ctx context.Context, userID, videoID uint) error {
	if userID == 0 || videoID == 0 {
		return nil
	}

	ref, err := s.activity.PublishedVideoAuthor(ctx, videoID)
	if err != nil {
		return err
	}

	if ref.VideoID == 0 || ref.AuthorID == 0 {
		return nil
	}

	if !s.follows.IsFollowing(ctx, userID, ref.AuthorID) {
		return nil
	}

	latest, ok, err := s.latestWork(ctx, ref.AuthorID)
	if err != nil || !ok {
		return err
	}

	if latest.WorkType != enum.WorkTypeVideo || latest.WorkID != videoID {
		return nil
	}

	return s.upsertRead(ctx, userID, ref.AuthorID, latest)
}

// latestWork 取作者的最新作品（视频与动态综合）。
//
// 与原 getLatestWork 一致：两条明细查询的错误被忽略（按「无作品」处理），
// 只有用户名查询失败或用户不存在才返回错误。
func (s *ReadService) latestWork(ctx context.Context, authorID uint) (domain.LatestWork, bool, error) {
	if authorID == 0 {
		return domain.LatestWork{}, false, domain.ErrAuthorIDRequired
	}

	username, err := s.users.UsernameByID(ctx, authorID)
	if err != nil {
		return domain.LatestWork{}, false, err
	}

	if username == "" {
		return domain.LatestWork{}, false, domain.ErrUserNotFound
	}

	video, _ := s.activity.LatestVideoWork(ctx, username)
	dynamic, _ := s.activity.LatestDynamicWork(ctx, authorID)

	latest, ok := domain.PickLatestWork(video, dynamic)

	return latest, ok, nil
}

// upsertRead 写入或推进已读位点，并在实际发生写入后失效未读数缓存。
func (s *ReadService) upsertRead(
	ctx context.Context,
	userID, authorID uint,
	latest domain.LatestWork,
) error {
	if userID == 0 || authorID == 0 || latest.WorkID == 0 || latest.CreatedAt.IsZero() {
		return nil
	}

	record, err := s.reads.FindByUserAndAuthor(ctx, userID, authorID)
	if err != nil {
		return err
	}

	if record == nil {
		if err := s.reads.Create(ctx, &domain.ReadRecord{
			UserID:           userID,
			AuthorID:         authorID,
			LastReadWorkType: latest.WorkType,
			LastReadWorkID:   latest.WorkID,
			LastReadWorkAt:   latest.CreatedAt,
		}); err != nil {
			return err
		}

		s.unread.InvalidateUnreadCount(userID)

		return nil
	}

	if !record.ShouldAdvance(latest.CreatedAt) {
		return nil
	}

	if err := s.reads.Advance(ctx, record.ID, latest.WorkType, latest.WorkID, latest.CreatedAt); err != nil {
		return err
	}

	s.unread.InvalidateUnreadCount(userID)

	return nil
}
