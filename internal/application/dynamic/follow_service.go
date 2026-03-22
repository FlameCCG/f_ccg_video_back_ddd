package dynamic

import (
	"context"
	"sort"
	"strconv"
	"time"

	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

// FollowService 关注流用例服务。
//
// 请求合并由实例内的 singleflight.Group 协调；缓存通过端口访问，未读数通过 InvalidateUnreadCount 失效。
type FollowService struct {
	follows  FollowRelationQuery
	activity ActivityQuery
	cache    UnreadFollowCountCache
	tasks    *ctxutil.DetachedTaskGroup
	countSFG singleflight.Group
	logger   *zap.Logger
	feeds    dynamicdomain.FeedPolicy
}

// NewFollowService 构造关注流用例服务。
func NewFollowService(
	follows FollowRelationQuery,
	activity ActivityQuery,
	cache UnreadFollowCountCache,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *FollowService {
	return &FollowService{
		follows:  follows,
		activity: activity,
		cache:    cache,
		tasks:    tasks,
		logger:   logger,
		feeds:    dynamicdomain.FeedPolicy{},
	}
}

// unreadCountKey 当前应用实例的并发合并键；Redis 键由缓存适配器维护。
func unreadCountKey(userID uint) string {
	return strconv.FormatUint(uint64(userID), 10)
}

// ListFollowUsers 分页列出当前用户的关注对象，并标注各自最新作品与未读状态。
//
// 排序在内存中完成：有最新作品的优先，其次按最新作品时间倒序；
// 都没有作品时按关注时间倒序；完全相同时按用户 ID 倒序。
func (s *FollowService) ListFollowUsers(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]FollowUserItem, int64, error) {
	if userID == 0 {
		return nil, 0, dynamicdomain.ErrUserNotLogin
	}

	page, pageSize = s.feeds.NormalizeFeedPaging(page, pageSize)

	follows, err := s.follows.ListFollowing(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	if len(follows) == 0 {
		return []FollowUserItem{}, 0, nil
	}

	focusIDs := make([]uint, 0, len(follows))

	usernames := make([]string, 0, len(follows))
	for _, f := range follows {
		focusIDs = append(focusIDs, f.FocusUserID)
		usernames = append(usernames, f.Username)
	}

	videoLatest, dynamicLatest, readLatest, err := s.loadFollowActivityMaps(ctx, userID, focusIDs, usernames)
	if err != nil {
		return nil, 0, err
	}

	items := make([]FollowUserItem, 0, len(follows))

	for _, f := range follows {
		activity := dynamicdomain.FollowActivity{
			LatestVideoAt:   videoLatest[f.Username],
			LatestDynamicAt: dynamicLatest[f.FocusUserID],
			LastReadAt:      readLatest[f.FocusUserID],
		}

		var latestAt *time.Time

		latestType := enum.WorkType(0)

		if at, workType, ok := activity.Latest(); ok {
			latestAt = &at
			latestType = workType
		}

		items = append(items, FollowUserItem{
			UserID:         f.FocusUserID,
			Username:       f.Username,
			Avatar:         f.Avatar,
			FollowedAt:     f.FollowedAt,
			LatestWorkAt:   latestAt,
			LatestWorkType: latestType,
			IsUnread:       activity.IsUnread(),
		})
	}

	sortFollowUsers(items)

	total := int64(len(items))

	start := (page - 1) * pageSize
	if start >= len(items) {
		return []FollowUserItem{}, total, nil
	}

	end := min(start+pageSize, len(items))

	return items[start:end], total, nil
}

// sortFollowUsers 关注列表内存排序。
func sortFollowUsers(items []FollowUserItem) {
	order := dynamicdomain.FollowOrder{}
	sort.SliceStable(items, func(i, j int) bool {
		return order.Less(dynamicdomain.FollowOrderFacts{UserID: items[i].UserID, LatestWorkAt: items[i].LatestWorkAt, FollowedAt: items[i].FollowedAt}, dynamicdomain.FollowOrderFacts{UserID: items[j].UserID, LatestWorkAt: items[j].LatestWorkAt, FollowedAt: items[j].FollowedAt})
	})
}

// UnreadFollowCount 返回当前用户关注对象中有未读作品的人数。
//
// 缓存命中且新鲜时直接返回；过期时先返回缓存再异步刷新；未命中时同步刷新，同一用户的并发刷新由 singleflight 合并。
func (s *FollowService) UnreadFollowCount(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, dynamicdomain.ErrUserNotLogin
	}

	if cached, fresh, ok := s.cache.Load(userID); ok {
		if fresh {
			return cached, nil
		}

		s.refreshUnreadCountAsync(userID)

		return cached, nil
	}

	return s.refreshUnreadCount(ctx, userID)
}

// InvalidateUnreadCount 未读位点推进后失效未读数缓存（含 singleflight 结果遗忘）。
func (s *FollowService) InvalidateUnreadCount(userID uint) {
	if userID == 0 {
		return
	}

	s.countSFG.Forget(unreadCountKey(userID))
	s.cache.Invalidate(userID)
}

func (s *FollowService) refreshUnreadCount(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, dynamicdomain.ErrUserNotLogin
	}

	value, err, _ := s.countSFG.Do(unreadCountKey(userID), func() (any, error) {
		if cached, fresh, ok := s.cache.Load(userID); ok && fresh {
			return cached, nil
		}

		count, err := s.unreadCountUncached(ctx, userID)
		if err != nil {
			return int64(0), err
		}

		s.cache.Store(userID, count)

		return count, nil
	})
	if err != nil {
		return 0, err
	}

	count, _ := value.(int64)

	return count, nil
}

func (s *FollowService) refreshUnreadCountAsync(userID uint) {
	s.tasks.Go(context.TODO(), ctxutil.AsyncQuickTimeout, func(bg context.Context) {
		if _, err := s.refreshUnreadCount(bg, userID); err != nil {
			s.logger.Warn("refresh unread follow count failed", zap.Uint("userId", userID), zap.Error(err))
		}
	})
}

func (s *FollowService) unreadCountUncached(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, dynamicdomain.ErrUserNotLogin
	}

	follows, err := s.follows.ListFollowing(ctx, userID)
	if err != nil {
		return 0, err
	}

	if len(follows) == 0 {
		return 0, nil
	}

	focusIDs := make([]uint, 0, len(follows))

	usernames := make([]string, 0, len(follows))
	for _, f := range follows {
		focusIDs = append(focusIDs, f.FocusUserID)
		usernames = append(usernames, f.Username)
	}

	videoLatest, dynamicLatest, readLatest, err := s.loadFollowActivityMaps(ctx, userID, focusIDs, usernames)
	if err != nil {
		return 0, err
	}

	var count int64

	for _, f := range follows {
		activity := dynamicdomain.FollowActivity{
			LatestVideoAt:   videoLatest[f.Username],
			LatestDynamicAt: dynamicLatest[f.FocusUserID],
			LastReadAt:      readLatest[f.FocusUserID],
		}
		if activity.IsUnread() {
			count++
		}
	}

	return count, nil
}

// loadFollowActivityMaps 并发加载「最新视频时间 / 最新动态时间 / 已读位点时间」三张映射表。
func (s *FollowService) loadFollowActivityMaps(
	ctx context.Context,
	userID uint,
	focusIDs []uint,
	usernames []string,
) (map[string]time.Time, map[uint]time.Time, map[uint]time.Time, error) {
	videoLatest := map[string]time.Time{}
	dynamicLatest := map[uint]time.Time{}

	readLatest := map[uint]time.Time{}
	if len(focusIDs) == 0 {
		return videoLatest, dynamicLatest, readLatest, nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error

		videoLatest, err = s.activity.LatestVideoTimes(groupCtx, usernames)

		return err
	})
	group.Go(func() error {
		var err error

		dynamicLatest, err = s.activity.LatestDynamicTimes(groupCtx, focusIDs)

		return err
	})
	group.Go(func() error {
		var err error

		readLatest, err = s.activity.LastReadTimes(groupCtx, userID, focusIDs)

		return err
	})

	if err := group.Wait(); err != nil {
		return nil, nil, nil, err
	}

	return videoLatest, dynamicLatest, readLatest, nil
}
