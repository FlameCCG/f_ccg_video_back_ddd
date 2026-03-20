package notification

import (
	"context"

	notificationentity "f_ccg_video_back_ddd/internal/domain/notification/entity"
	notificationrepo "f_ccg_video_back_ddd/internal/domain/notification/repository"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Create 写入通知并失效角标缓存。
func (s *DeliveryService) Create(ctx context.Context, notice *notificationentity.Notification) error {
	if err := s.repo.Create(ctx, notice); err != nil {
		return err
	}

	s.counts.InvalidateCount(notice.ReceiverID)

	return nil
}

// NotifyBanStatus 发送账号状态变更系统通知，实现 identity 声明的 BanNotifier 端口。
//
// 通知写入失败时静默忽略，不影响封禁主流程。
func (s *DeliveryService) NotifyBanStatus(
	ctx context.Context,
	receiverID uint,
	actionUserID uint,
	actionUserName, actionUserAvatar, title, content string,
) {
	notice := &notificationentity.Notification{
		Type:             enum.NotificationSystemType,
		ReceiverID:       int(receiverID),
		ActionUserID:     actionUserID,
		ActionUserName:   actionUserName,
		ActionUserAvatar: actionUserAvatar,
		Title:            title,
		Content:          content,
	}

	if err := s.repo.Create(ctx, notice); err == nil {
		s.counts.InvalidateCount(notice.ReceiverID)
	}
}

// CreateBatch 批量写入通知，并逐个失效对应接收者的角标缓存。
func (s *DeliveryService) CreateBatch(ctx context.Context, notices []notificationentity.Notification) error {
	if len(notices) == 0 {
		return nil
	}

	if err := s.repo.CreateBatch(ctx, notices); err != nil {
		return err
	}

	receivers := make([]int, 0, len(notices))
	for i := range notices {
		receivers = append(receivers, notices[i].ReceiverID)
	}

	s.counts.invalidateCountForReceivers(receivers)

	return nil
}

// DeliveryService 负责通知写入与投递。
type DeliveryService struct {
	repo   notificationrepo.Repository
	counts *CountService
}

func NewDeliveryService(repo notificationrepo.Repository, counts *CountService) *DeliveryService {
	return &DeliveryService{repo: repo, counts: counts}
}
