package notification

import (
	"context"
	"errors"
	"fmt"

	notificationdomain "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// 管理端更新系统通知的失败分类，对应不同响应文案。
var (
	// ErrNoticeQueryFailed 读取目标通知失败，响应文案为「查询失败」。
	ErrNoticeQueryFailed = errors.New("查询失败")
	// ErrNoticeNotFound 表示通知不存在，响应文案为「通知不存在」。
	ErrNoticeNotFound = errors.New("通知不存在")
	// ErrNoticeUpdateFailed 表示写入更新失败，响应文案为「更新失败」。
	ErrNoticeUpdateFailed = errors.New("更新失败")
)

// AdminDelete 仅管理员使用，直接按 ID 删除内容及关联已读状态。
func (s *AdminService) AdminDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	var receivers []int
	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		notices, err := s.repo.FindByIDsForUpdate(txCtx, ids)
		if err != nil {
			return err
		}
		for _, notice := range notices {
			receivers = append(receivers, notice.ReceiverID())
		}
		return s.repo.DeleteByIDs(txCtx, ids)
	}); err != nil {
		return err
	}

	s.counts.invalidateCountForReceivers(receivers)

	return nil
}

// AdminList 管理员查看系统通知。
func (s *AdminService) AdminList(ctx context.Context, page, pageSize int) ([]AdminNotification, int64, error) {
	page, pageSize = (notificationdomain.PagePolicy{}).Normalize(page, pageSize)

	list, total, err := s.repo.ListSystem(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return s.adminNotifications(list), total, nil
}

func (s *AdminService) adminNotifications(list []notificationdomain.Notification) []AdminNotification {
	if list == nil {
		return nil
	}

	out := make([]AdminNotification, len(list))
	for i := range list {
		snap := list[i].Snapshot()
		out[i] = AdminNotification{
			ID:               snap.ID,
			Type:             snap.Type,
			ReceiverID:       snap.ReceiverID,
			ActionUserID:     snap.ActionUserID,
			ActionUserAvatar: snap.ActionUserAvatar,
			ActionUserName:   snap.ActionUserName,
			Title:            snap.Title,
			Content:          snap.Content,
			Link:             snap.Link,
			VideoID:          snap.VideoID,
			VideoTitle:       snap.VideoTitle,
			DynamicID:        snap.DynamicID,
			CommentID:        snap.CommentID,
		}
	}

	return out
}

// CreateSystemNotice 管理端发布系统通知。
//
// 操作者的用户名/头像取自读侧投影；类型固定为系统通知。
func (s *AdminService) CreateSystemNotice(ctx context.Context, cmd CreateSystemNoticeCommand) error {
	actionName, actionAvatar := "", ""
	if s.actors != nil {
		actionName, actionAvatar = s.actors.Profile(ctx, cmd.ActorID)
	}

	notice := notificationdomain.NewNotification(notificationdomain.Draft{
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
	})

	saved, err := s.repo.Create(ctx, notice)
	if err != nil {
		return err
	}
	s.counts.InvalidateCount(saved.ReceiverID())
	return nil
}

// UpdateSystemNotice 管理端更新系统通知。
//
// 在同一事务内锁定最新记录并修改，提交后失效新旧接收者的角标缓存。
// 通知不存在时返回 ErrNoticeNotFound，保留原读取与写入错误分类。
func (s *AdminService) UpdateSystemNotice(ctx context.Context, cmd UpdateSystemNoticeCommand) error {
	var previousReceiverID int
	started := false
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		started = true
		notices, err := s.repo.FindByIDsForUpdate(txCtx, []uint{cmd.ID})
		if err != nil {
			return fmt.Errorf("%w: %w", ErrNoticeQueryFailed, err)
		}
		if len(notices) == 0 {
			return ErrNoticeNotFound
		}
		notice := &notices[0]
		notice.ReviseAsSystem(notificationdomain.SystemContent{
			Title: cmd.Title, Content: cmd.Content, ReceiverID: cmd.ReceiverID,
			VideoID: cmd.VideoID, VideoTitle: cmd.VideoTitle, Link: cmd.Link,
		})
		previousReceiverID, err = s.repo.UpdateSystemContent(txCtx, notice)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrNoticeUpdateFailed, err)
		}
		return nil
	})
	if err != nil {
		if !started {
			return fmt.Errorf("%w: %w", ErrNoticeQueryFailed, err)
		}
		if errors.Is(err, ErrNoticeQueryFailed) || errors.Is(err, ErrNoticeNotFound) || errors.Is(err, ErrNoticeUpdateFailed) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrNoticeUpdateFailed, err)
	}
	s.counts.invalidateCountForReceivers([]int{previousReceiverID, cmd.ReceiverID})
	return nil
}

// AdminService 负责系统通知管理。
type AdminService struct {
	txm    tx.Manager
	repo   notificationdomain.Repository
	actors ActorProfileProvider
	counts *CountService
}

func NewAdminService(txm tx.Manager, repo notificationdomain.Repository, actors ActorProfileProvider, counts *CountService) *AdminService {
	return &AdminService{txm: txm, repo: repo, actors: actors, counts: counts}
}
