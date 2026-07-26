package notification

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/notification"
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

// Service 通知用例服务（原 service/notification_service）。
//
// 原实现使用包级 singleflight.Group + 包级函数
// InvalidateNotificationCountCache 供跨包调用；新架构改为实例字段与显式端口。
type Service struct {
	repo     domain.Repository
	cache    CountCache
	chat     ChatUnreadCounter
	actors   ActorProfileProvider
	tasks    *ctxutil.DetachedTaskGroup
	countSFG singleflight.Group
}

// 管理端更新系统通知的失败分类（对应原处理器三种不同的响应文案）。
var (
	// ErrNoticeQueryFailed 读取原通知失败（原文案「查询失败」）。
	ErrNoticeQueryFailed = errors.New("查询失败")
	// ErrNoticeNotFound 通知不存在（原文案「通知不存在」）。
	ErrNoticeNotFound = errors.New("通知不存在")
	// ErrNoticeUpdateFailed 写入更新失败（原文案「更新失败」）。
	ErrNoticeUpdateFailed = errors.New("更新失败")
)

// NewService 构造通知用例服务。
func NewService(
	repo domain.Repository,
	cache CountCache,
	chat ChatUnreadCounter,
	actors ActorProfileProvider,
	tasks *ctxutil.DetachedTaskGroup,
) *Service {
	return &Service{repo: repo, cache: cache, chat: chat, actors: actors, tasks: tasks}
}

// Create 写入通知并失效角标缓存。
func (s *Service) Create(ctx context.Context, notice *domain.Notification) error {
	if err := s.repo.Create(ctx, notice); err != nil {
		return err
	}

	s.invalidateCount(notice.ReceiverID)

	return nil
}

// NotifyBanStatus 发送账号状态变更系统通知。
//
// 实现 identity 上下文声明的 BanNotifier 端口（跨上下文协调在应用层完成）。
// 与原 service/user_service/ban_service.go 的 notifyBanStatus 尾段一致：
// 写入失败时静默忽略（不影响封禁主流程）。
func (s *Service) NotifyBanStatus(
	ctx context.Context,
	receiverID uint,
	actionUserID uint,
	actionUserName, actionUserAvatar, title, content string,
) {
	notice := &domain.Notification{
		Type:             enum.NotificationSystemType,
		ReceiverID:       int(receiverID),
		ActionUserID:     actionUserID,
		ActionUserName:   actionUserName,
		ActionUserAvatar: actionUserAvatar,
		Title:            title,
		Content:          content,
	}

	if err := s.repo.Create(ctx, notice); err == nil {
		s.invalidateCount(notice.ReceiverID)
	}
}

// List 用户通知分页列表；isRead 由 notification_user_reads 推导（有行 = 已读）。
func (s *Service) List(
	ctx context.Context,
	receiverID, page, pageSize int,
	category string,
) ([]NotificationItem, int64, error) {
	page, pageSize = normalizePaging(page, pageSize)

	types := domain.TypesByCategory(category)

	list, total, err := s.repo.ListForReceiver(ctx, receiverID, types, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	readSet, err := s.repo.ReadSet(ctx, uint(receiverID), notificationIDs(list))
	if err != nil {
		return nil, 0, err
	}

	items := make([]NotificationItem, 0, len(list))

	for _, n := range list {
		_, isRead := readSet[n.ID]
		items = append(items, NotificationItem{
			ID:               n.ID,
			Type:             n.Type,
			ReceiverID:       n.ReceiverID,
			ActionUserID:     n.ActionUserID,
			ActionUserAvatar: n.ActionUserAvatar,
			ActionUserName:   n.ActionUserName,
			Title:            n.Title,
			Content:          n.Content,
			VideoID:          n.VideoID,
			VideoTitle:       n.VideoTitle,
			Link:             n.Link,
			CommentID:        n.CommentID,
			DynamicID:        n.DynamicID,
			IsRead:           isRead,
			CreatedAt:        n.CreatedAt,
		})
	}

	return items, total, nil
}

// MarkRead 将指定通知标记为已读（插入 read 行；个人 + 全员 -1 均可）。
func (s *Service) MarkRead(ctx context.Context, receiverID int, ids []uint) error {
	if len(ids) == 0 || receiverID <= 0 {
		return nil
	}

	validIDs, err := s.repo.VisibleIDs(ctx, receiverID, ids)
	if err != nil {
		return err
	}

	if len(validIDs) == 0 {
		return nil
	}

	if err := s.repo.MarkRead(ctx, receiverID, validIDs); err != nil {
		return err
	}

	s.invalidateCount(receiverID)

	return nil
}

// Delete 硬删个人非系统通知；系统消息（含全员 -1）禁止删除。
func (s *Service) Delete(ctx context.Context, receiverID int, ids []uint) error {
	if len(ids) == 0 || receiverID <= 0 {
		return nil
	}

	blocked, err := s.repo.CountUndeletable(ctx, ids)
	if err != nil {
		return err
	}

	if blocked > 0 {
		return domain.ErrSystemNotificationNotDeletable
	}

	deletableIDs, err := s.repo.DeletableIDs(ctx, receiverID, ids)
	if err != nil {
		return err
	}

	if len(deletableIDs) == 0 {
		return nil
	}

	if err := s.repo.DeleteForReceiver(ctx, receiverID, deletableIDs); err != nil {
		return err
	}

	s.invalidateCount(receiverID)

	return nil
}

// AdminDelete 仅管理员使用，直接按 ID 删除内容及关联已读状态。
func (s *Service) AdminDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	receivers, err := s.repo.DistinctReceivers(ctx, ids)
	if err != nil {
		return err
	}

	if err := s.repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}

	s.invalidateCountForReceivers(receivers)

	return nil
}

// AdminList 管理员查看系统通知。
func (s *Service) AdminList(ctx context.Context, page, pageSize int) ([]domain.Notification, int64, error) {
	page, pageSize = normalizePaging(page, pageSize)

	return s.repo.ListSystem(ctx, page, pageSize)
}

// GetAdmin 按 ID 读取通知；不存在时返回 (nil, nil)。
func (s *Service) GetAdmin(ctx context.Context, id uint) (*domain.Notification, error) {
	return s.repo.FindByID(ctx, id)
}

// AdminUpdate 管理员更新系统通知。
func (s *Service) AdminUpdate(ctx context.Context, notice *domain.Notification) error {
	previousReceiverID, err := s.repo.UpdateSystemContent(ctx, notice)
	if err != nil {
		return err
	}

	s.invalidateCountForReceivers([]int{previousReceiverID, notice.ReceiverID})

	return nil
}

// CountSummary 角标计数汇总（fresh 直接返回；stale 返回旧值并异步刷新）。
func (s *Service) CountSummary(ctx context.Context, receiverID int) (*NotificationCountSummary, error) {
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
func (s *Service) InvalidateCount(receiverID int) {
	s.invalidateCount(receiverID)
}

// invalidateCount 写后失效，分支顺序与原 invalidateNotificationCountCache 完全一致。
func (s *Service) invalidateCount(receiverID int) {
	if receiverID == 0 {
		return
	}

	if !s.cache.Available() {
		if receiverID != domain.BroadcastReceiverID {
			s.countSFG.Forget(countSFKey(receiverID))
		}

		return
	}

	if receiverID == domain.BroadcastReceiverID {
		s.cache.InvalidateAll()

		return
	}

	s.countSFG.Forget(countSFKey(receiverID))
	s.cache.Invalidate(receiverID)
}

// invalidateCountForReceivers 去重后失效；含 -1 时走全量清理。
func (s *Service) invalidateCountForReceivers(receivers []int) {
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

func (s *Service) refreshCountSummary(ctx context.Context, receiverID int) (*NotificationCountSummary, error) {
	value, err, _ := s.countSFG.Do(countSFKey(receiverID), func() (any, error) {
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

func (s *Service) refreshCountSummaryAsync(receiverID int) {
	s.tasks.Go(context.TODO(), ctxutil.AsyncQuickTimeout, func(bg context.Context) {
		if _, err := s.refreshCountSummary(bg, receiverID); err != nil {
			zap.L().Warn("refresh notification count cache failed", zap.Int("receiverId", receiverID), zap.Error(err))
		}
	})
}

func (s *Service) loadCountSummary(ctx context.Context, receiverID int) (NotificationCountSummary, error) {
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
			zap.L().Warn("load chat unread failed", zap.Int("receiverId", receiverID), zap.Error(err))

			return nil
		}

		message = n

		return nil
	})

	if err := group.Wait(); err != nil {
		return NotificationCountSummary{}, err
	}

	return NotificationCountSummary{
		Reply: groupedCounts[enum.NotificationApplyType],
		Like: groupedCounts[enum.NotificationVideoLikeType] +
			groupedCounts[enum.NotificationLikeCommentType] +
			groupedCounts[enum.NotificationLikeDynamicType],
		At:      groupedCounts[enum.NotificationAtType],
		System:  groupedCounts[enum.NotificationSystemType],
		Message: message,
	}, nil
}

func countSFKey(receiverID int) string {
	return strconv.Itoa(receiverID)
}

func notificationIDs(list []domain.Notification) []uint {
	ids := make([]uint, 0, len(list))

	for _, n := range list {
		if n.ID != 0 {
			ids = append(ids, n.ID)
		}
	}

	return ids
}

func normalizePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}

// CreateSystemNotice 管理端发布系统通知。
//
// 操作者的用户名/头像取自读侧投影；类型固定为系统通知（与原
// api/notification_api/create.go 一致）。
func (s *Service) CreateSystemNotice(ctx context.Context, cmd CreateSystemNoticeCommand) error {
	actionName, actionAvatar := "", ""
	if s.actors != nil {
		actionName, actionAvatar = s.actors.Profile(ctx, cmd.ActorID)
	}

	notice := &domain.Notification{
		// 管理员发布的系统消息固定为系统类型
		Type:             enum.NotificationSystemType,
		ReceiverID:       cmd.ReceiverID,
		Title:            cmd.Title,
		Content:          cmd.Content,
		Link:             cmd.Link,
		VideoID:          cmd.VideoID,
		VideoTitle:       cmd.VideoTitle,
		ActionUserID:     cmd.ActorID,
		ActionUserName:   actionName,
		ActionUserAvatar: actionAvatar,
	}

	return s.Create(ctx, notice)
}

// UpdateSystemNotice 管理端更新系统通知。
//
// 先读原记录，覆盖可编辑字段后强制置为系统通知类型（与原
// api/notification_api/admin_update.go 一致）。通知不存在时返回 ErrNoticeNotFound。
func (s *Service) UpdateSystemNotice(ctx context.Context, cmd UpdateSystemNoticeCommand) error {
	old, err := s.GetAdmin(ctx, cmd.ID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNoticeQueryFailed, err)
	}

	if old == nil {
		return ErrNoticeNotFound
	}

	old.Title = cmd.Title
	old.Content = cmd.Content
	old.ReceiverID = cmd.ReceiverID
	old.VideoID = cmd.VideoID
	old.VideoTitle = cmd.VideoTitle
	old.Link = cmd.Link
	// 强制为系统通知
	old.Type = enum.NotificationSystemType

	if err := s.AdminUpdate(ctx, old); err != nil {
		return fmt.Errorf("%w: %w", ErrNoticeUpdateFailed, err)
	}

	return nil
}
