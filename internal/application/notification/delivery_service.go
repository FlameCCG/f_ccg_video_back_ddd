package notification

import (
	"context"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Create 写入通知并失效角标缓存。
func (s *DeliveryService) Create(ctx context.Context, notice *notificationdomain.Notification) (*notificationdomain.Notification, error) {
	saved, err := s.repo.Create(ctx, notice)
	if err != nil {
		return nil, err
	}

	s.counts.InvalidateCount(saved.ReceiverID())

	return saved, nil
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
	notice := notificationdomain.NewNotification(notificationdomain.Draft{
		Type:             enum.NotificationSystemType,
		ReceiverID:       int(receiverID),
		ActionUserID:     actionUserID,
		ActionUserName:   actionUserName,
		ActionUserAvatar: actionUserAvatar,
		Title:            title,
		Content:          content,
	})

	saved, err := s.repo.Create(ctx, notice)
	if err == nil {
		s.counts.InvalidateCount(saved.ReceiverID())
	}
}

// CreateBatch 批量写入通知，并逐个失效对应接收者的角标缓存。
func (s *DeliveryService) CreateBatch(ctx context.Context, notices []notificationdomain.Notification) ([]notificationdomain.Notification, error) {
	if len(notices) == 0 {
		return nil, nil
	}

	saved, err := s.repo.CreateBatch(ctx, notices)
	if err != nil {
		return nil, err
	}

	receivers := make([]int, 0, len(saved))
	for i := range saved {
		receivers = append(receivers, saved[i].ReceiverID())
	}

	s.counts.invalidateCountForReceivers(receivers)

	return saved, nil
}

// DeliveryService 负责通知写入与投递。
type DeliveryService struct {
	repo   notificationdomain.Repository
	counts *CountService
}

func NewDeliveryService(repo notificationdomain.Repository, counts *CountService) *DeliveryService {
	return &DeliveryService{repo: repo, counts: counts}
}
