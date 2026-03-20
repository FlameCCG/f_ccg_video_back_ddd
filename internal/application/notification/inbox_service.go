package notification

import (
	"context"

	notificationentity "f_ccg_video_back_ddd/internal/domain/notification/entity"
	notificationrepo "f_ccg_video_back_ddd/internal/domain/notification/repository"
)

// List 用户通知分页列表；isRead 由 notification_user_reads 推导（有行 = 已读）。
func (s *InboxService) List(
	ctx context.Context,
	receiverID, page, pageSize int,
	category string,
) ([]NotificationItem, int64, error) {
	page, pageSize = normalizePaging(page, pageSize)

	types := notificationentity.TypesByCategory(category)

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
func (s *InboxService) MarkRead(ctx context.Context, receiverID int, ids []uint) error {
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

	s.counts.InvalidateCount(receiverID)

	return nil
}

// Delete 硬删个人非系统通知；系统消息（含全员 -1）禁止删除。
func (s *InboxService) Delete(ctx context.Context, receiverID int, ids []uint) error {
	if len(ids) == 0 || receiverID <= 0 {
		return nil
	}

	blocked, err := s.repo.CountUndeletable(ctx, ids)
	if err != nil {
		return err
	}

	if blocked > 0 {
		return notificationentity.ErrSystemNotificationNotDeletable
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

	s.counts.InvalidateCount(receiverID)

	return nil
}

func notificationIDs(list []notificationentity.Notification) []uint {
	ids := make([]uint, 0, len(list))

	for _, n := range list {
		if n.ID != 0 {
			ids = append(ids, n.ID)
		}
	}

	return ids
}

// InboxService 负责接收者收件箱与已读状态。
type InboxService struct {
	repo   notificationrepo.Repository
	counts *CountService
}

func NewInboxService(repo notificationrepo.Repository, counts *CountService) *InboxService {
	return &InboxService{repo: repo, counts: counts}
}
