package notification

import (
	"context"
	"strconv"
	"time"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	notificationCountDBTimeout   = 2 * time.Second
	notificationCountChatTimeout = 800 * time.Millisecond
)

// CountSummary 角标计数汇总（fresh 直接返回；stale 返回旧值并异步刷新）。
//
// 计数是展示读模型，允许短时陈旧；通知正文、每用户已读状态与私信未读事实分别来自其查询端口。
// 缓存的 fresh/stale 状态决定响应等待策略，不能用角标值裁定消息是否存在或能否读取。
func (s *CountService) CountSummary(ctx context.Context, receiverID int) (*NotificationCountSummary, error) {
	if cached, fresh, ok := s.cache.Load(receiverID); ok {
		if fresh {
			return cached, nil
		}

		s.refreshCountSummaryAsync(receiverID)

		return cached, nil
	}

	return s.refreshCountSummary(ctx, receiverID)
}

// InvalidateCount 写通知/聊天未读变化后失效角标缓存（跨上下文调用入口）。
// receiverID == -1（全员通知）时清除全部计数缓存。
func (s *CountService) InvalidateCount(receiverID int) {
	s.invalidateCount(receiverID)
}

// invalidateCount 在写入后使通知计数缓存失效。
func (s *CountService) invalidateCount(receiverID int) {
	if receiverID == 0 {
		return
	}

	if !s.cache.Available() {
		if receiverID != notificationdomain.BroadcastReceiverID {
			s.countSFG.Forget(s.countSFKey(receiverID))
		}

		return
	}

	if receiverID == notificationdomain.BroadcastReceiverID {
		s.cache.InvalidateAll()

		return
	}

	s.countSFG.Forget(s.countSFKey(receiverID))
	s.cache.Invalidate(receiverID)
}

// invalidateCountForReceivers 去重后失效；含 -1 时走全量清理。
func (s *CountService) invalidateCountForReceivers(receivers []int) {
	seen := make(map[int]struct{}, len(receivers))

	for _, id := range receivers {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		s.invalidateCount(id)
	}
}

// refreshCountSummary 用本 CountService 实例内的 singleflight 合并同一接收者的并发重建。
// 它避免本进程重复查库，不是多实例分布式锁；回调内再次检查 fresh 缓存以复用刚完成的结果。
func (s *CountService) refreshCountSummary(ctx context.Context, receiverID int) (*NotificationCountSummary, error) {
	value, err, _ := s.countSFG.Do(s.countSFKey(receiverID), func() (any, error) {
		if cached, fresh, ok := s.cache.Load(receiverID); ok && fresh {
			return *cached, nil
		}

		result, err := s.loadCountSummary(ctx, receiverID)
		if err != nil {
			return NotificationCountSummary{}, err
		}

		s.cache.Store(receiverID, result)

		return result, nil
	})
	if err != nil {
		return nil, err
	}

	result, _ := value.(NotificationCountSummary)

	return &result, nil
}

func (s *CountService) refreshCountSummaryAsync(receiverID int) {
	s.tasks.Go(context.TODO(), ctxutil.AsyncQuickTimeout, func(bg context.Context) {
		if _, err := s.refreshCountSummary(bg, receiverID); err != nil {
			s.logger.Warn("refresh notification count cache failed", zap.Int("receiverId", receiverID), zap.Error(err))
		}
	})
}

func (s *CountService) loadCountSummary(ctx context.Context, receiverID int) (NotificationCountSummary, error) {
	if ctx == nil {
		ctx = context.TODO()
	}

	var (
		groupedCounts map[enum.Type]int64
		message       int64
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		dbCtx, cancel := context.WithTimeout(groupCtx, notificationCountDBTimeout)
		defer cancel()

		var err error

		groupedCounts, err = s.repo.CountUnreadGrouped(dbCtx, receiverID)

		return err
	})
	group.Go(func() error {
		chatCtx, cancel := context.WithTimeout(groupCtx, notificationCountChatTimeout)
		defer cancel()

		n, err := s.chat.TotalUnread(chatCtx, uint(receiverID))
		if err != nil {
			s.logger.Warn("load chat unread failed", zap.Int("receiverId", receiverID), zap.Error(err))

			return nil
		}

		message = n

		return nil
	})

	if err := group.Wait(); err != nil {
		return NotificationCountSummary{}, err
	}

	return NotificationCountSummary{
		Reply:   s.types.SumByCategory(groupedCounts, "reply"),
		Like:    s.types.SumByCategory(groupedCounts, "like"),
		At:      s.types.SumByCategory(groupedCounts, "at"),
		System:  s.types.SumByCategory(groupedCounts, "system"),
		Message: message,
	}, nil
}

func (s *CountService) countSFKey(receiverID int) string {
	return strconv.Itoa(receiverID)
}

// CountService 负责角标缓存、未读组合和后台刷新。
type CountService struct {
	repo     notificationdomain.Repository
	cache    CountCache
	chat     ChatUnreadCounter
	tasks    *ctxutil.DetachedTaskGroup
	countSFG singleflight.Group
	logger   *zap.Logger
	types    notificationdomain.TypePolicy
}

func NewCountService(repo notificationdomain.Repository, cache CountCache, chat ChatUnreadCounter, tasks *ctxutil.DetachedTaskGroup, logger *zap.Logger) *CountService {
	return &CountService{repo: repo, cache: cache, chat: chat, tasks: tasks, logger: logger, types: notificationdomain.TypePolicy{}}
}
