package identity

import (
	"context"
	"fmt"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// banNotifyTitle 封禁通知标题（与原实现一致）。
const banNotifyTitle = "账号状态变更通知"

// BanService 封禁用例服务（原 service/user_service/ban_service.go）。
type BanService struct {
	txm      tx.Manager
	users    domain.UserRepository
	records  domain.BanRecordRepository
	statuses *UserStatusService
	notifier BanNotifier
}

// NewBanService 构造封禁用例服务。
func NewBanService(
	txm tx.Manager,
	users domain.UserRepository,
	records domain.BanRecordRepository,
	statuses *UserStatusService,
	notifier BanNotifier,
) *BanService {
	return &BanService{txm: txm, users: users, records: records, statuses: statuses, notifier: notifier}
}

// Apply 执行封禁/解封，并写状态缓存与站内通知。
func (s *BanService) Apply(ctx context.Context, action domain.BanAction) (*domain.BanRecord, error) {
	normalized, err := domain.NormalizeBanAction(action)
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindStatusByID(ctx, normalized.UserID)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	record := domain.NewBanRecord(normalized, time.Now())

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if updateErr := s.users.UpdateStatus(txCtx, user.ID, normalized.Status); updateErr != nil {
			return updateErr
		}

		if normalized.Status != enum.UserStatusActive {
			return s.records.Create(txCtx, record)
		}

		latest, findErr := s.records.FindLatestUnreleasedForUpdate(
			txCtx,
			user.ID,
			[]enum.UserStatus{
				enum.UserStatusBanned,
				enum.UserStatusPermanentlyBanned,
			},
		)
		if findErr != nil {
			return findErr
		}

		if latest == nil {
			return nil
		}

		if markErr := s.records.MarkUnbanned(txCtx, latest.ID); markErr != nil {
			return markErr
		}

		record.ID = latest.ID

		return nil
	}); err != nil {
		return nil, err
	}

	// P1-3: 权威 write-through，禁止仅 DEL 后被在途旧 Active 回填。
	s.statuses.Write(user.ID, normalized.Status)
	s.notifyBanStatus(ctx, user, record)

	return record, nil
}

func (s *BanService) notifyBanStatus(ctx context.Context, user *domain.User, record *domain.BanRecord) {
	if user == nil || user.ID == 0 || record == nil || s.notifier == nil {
		return
	}

	content := banStatusContent(record)

	var (
		actionUserID     uint
		actionUserName   string
		actionUserAvatar string
	)

	if record.OperatorID > 0 {
		operator, err := s.users.FindProfileByID(ctx, record.OperatorID)
		if err == nil && operator != nil {
			actionUserID = operator.ID
			actionUserName = operator.Username
			actionUserAvatar = operator.Avatar
		} else {
			actionUserID = record.OperatorID
		}
	}

	s.notifier.NotifyBanStatus(ctx, user.ID, actionUserID, actionUserName, actionUserAvatar, banNotifyTitle, content)
}

// banStatusContent 组装封禁通知正文（文案与原实现逐字一致）。
func banStatusContent(record *domain.BanRecord) string {
	content := ""

	switch record.Status {
	case enum.UserStatusActive:
		content = "您的账号已解除封禁。"
	case enum.UserStatusBanned:
		content = fmt.Sprintf("您的账号已被封禁 %d 天。", record.Days)
		if record.EndAt != nil {
			content += fmt.Sprintf("解封时间：%s。", record.EndAt.Format("2006-01-02 15:04:05"))
		}
	case enum.UserStatusPermanentlyBanned:
		content = "您的账号已被永久封禁。"
	}

	if record.Reason != "" {
		content += "原因：" + record.Reason
	}

	return content
}

// ReleaseExpired 原子地解封到期用户并标记原封禁记录，返回本次是否完成解封。
func (s *BanService) ReleaseExpired(ctx context.Context, userID uint, now time.Time) (bool, error) {
	if userID == 0 {
		return false, domain.ErrBanUserIDRequired
	}

	unbanned := false
	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		record, findErr := s.records.FindLatestUnreleasedForUpdate(
			txCtx,
			userID,
			[]enum.UserStatus{enum.UserStatusBanned},
		)
		if findErr != nil {
			return findErr
		}

		if record == nil || record.EndAt == nil || record.EndAt.After(now) {
			return nil
		}

		activated, activateErr := s.users.ActivateIfBanned(txCtx, userID)
		if activateErr != nil {
			return activateErr
		}

		if !activated {
			return nil
		}

		if markErr := s.records.MarkUnbanned(txCtx, record.ID); markErr != nil {
			return markErr
		}

		unbanned = true

		return nil
	}); err != nil {
		return false, err
	}

	if unbanned {
		// P1-3: 到期解封 write-through Active。
		s.statuses.Write(userID, enum.UserStatusActive)
	}

	return unbanned, nil
}

// BanRecordService 封禁记录查询用例服务（原 service/user_service/ban_record_service.go
// 与 LatestBanRecord）。
type BanRecordService struct {
	records domain.BanRecordRepository
	query   BanRecordQuery
}

// NewBanRecordService 构造封禁记录查询服务。
func NewBanRecordService(records domain.BanRecordRepository, query BanRecordQuery) *BanRecordService {
	return &BanRecordService{records: records, query: query}
}

// Latest 读取用户最近一条未解封封禁记录；无记录时返回 (nil, nil)。
func (s *BanRecordService) Latest(ctx context.Context, userID uint) (*domain.BanRecord, error) {
	if userID == 0 {
		return nil, domain.ErrBanUserIDRequired
	}

	return s.records.FindLatestByUserID(ctx, userID)
}

// AdminList 管理端封禁记录分页列表。
func (s *BanRecordService) AdminList(
	ctx context.Context,
	params AdminBanRecordListParams,
) ([]AdminBanRecordItem, int64, error) {
	return s.query.AdminList(ctx, params)
}

// EnsureCanPublish rechecks DB status for every content write, including long-lived sockets.
func (s *BanService) EnsureCanPublish(ctx context.Context, userID uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}
	user, err := s.users.FindStatusByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return domain.ErrUserNotFound
	}
	if user.Status == enum.UserStatusBanned {
		if _, err := s.ReleaseExpired(ctx, userID, time.Now()); err != nil {
			return err
		}
		user, err = s.users.FindStatusByID(ctx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			return domain.ErrUserNotFound
		}
	}
	if user.IsPermanentlyBanned() {
		return domain.ErrAccountPermanentlyBanned
	}
	if user.IsBanned() {
		return domain.ErrAccountBanned
	}
	return nil
}
