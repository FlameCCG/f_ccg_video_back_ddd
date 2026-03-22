package notification

import (
	"context"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
)

// List 用户通知分页列表；isRead 由 notification_user_reads 推导（有行 = 已读）。
func (s *InboxService) List(
	ctx context.Context,
	receiverID, page, pageSize int,
	category string,
) ([]NotificationItem, int64, error) {
	page, pageSize = (notificationdomain.PagePolicy{}).Normalize(page, pageSize)

	types := s.types.TypesByCategory(category)

	list, total, err := s.repo.ListForReceiver(ctx, receiverID, types, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	readSet, err := s.repo.ReadSet(ctx, uint(receiverID), s.notificationIDs(list))
	if err != nil {
		return nil, 0, err
	}

	items := make([]NotificationItem, 0, len(list))

	for _, n := range list {
		_, isRead := readSet[n.ID()]
		snap := n.Snapshot()
		items = append(items, NotificationItem{
			ID:               snap.ID,
			Type:             snap.Type,
			ReceiverID:       snap.ReceiverID,
			ActionUserID:     snap.ActionUserID,
			ActionUserAvatar: snap.ActionUserAvatar,
			ActionUserName:   snap.ActionUserName,
			Title:            snap.Title,
			Content:          snap.Content,
			VideoID:          snap.VideoID,
			VideoTitle:       snap.VideoTitle,
			Link:             snap.Link,
			CommentID:        snap.CommentID,
			DynamicID:        snap.DynamicID,
			IsRead:           isRead,
			CreatedAt:        snap.CreatedAt,
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

	notices, err := s.repo.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}

	deletableIDs := make([]uint, 0, len(notices))
	for i := range notices {
		if err := notices[i].EnsureDeletable(); err != nil {
			return err
		}

		if notices[i].DeletableBy(receiverID) {
			deletableIDs = append(deletableIDs, notices[i].ID())
		}
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

func (s *InboxService) notificationIDs(list []notificationdomain.Notification) []uint {
	ids := make([]uint, 0, len(list))

	for i := range list {
		if id := list[i].ID(); id != 0 {
			ids = append(ids, id)
		}
	}

	return ids
}

// InboxService 负责接收者收件箱与已读状态。
type InboxService struct {
	repo   notificationdomain.Repository
	counts *CountService
	types  notificationdomain.TypePolicy
}

func NewInboxService(repo notificationdomain.Repository, counts *CountService) *InboxService {
	return &InboxService{repo: repo, counts: counts, types: notificationdomain.TypePolicy{}}
}
