package identity

import (
	"context"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// banNotifyTitle 封禁通知标题。
const banNotifyTitle = "账号状态变更通知"

// BanService 封禁用例服务。
type BanService struct {
	txm      tx.Manager
	users    identitydomain.UserRepository
	records  identitydomain.BanRecordRepository
	statuses *UserStatusService
	notifier BanNotifier
}

// NewBanService 构造封禁用例服务。
func NewBanService(
	txm tx.Manager,
	users identitydomain.UserRepository,
	records identitydomain.BanRecordRepository,
	statuses *UserStatusService,
	notifier BanNotifier,
) *BanService {
	return &BanService{txm: txm, users: users, records: records, statuses: statuses, notifier: notifier}
}

// Apply 执行封禁/解封，并写状态缓存与站内通知。
func (s *BanService) Apply(ctx context.Context, action identitydomain.BanAction) (*BanRecordDTO, error) {
	normalized, err := action.Normalize()
	if err != nil {
		return nil, err
	}

	var user *identitydomain.User
	var record *identitydomain.BanRecord
	var releasedRecordID uint

	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		var findErr error
		user, findErr = s.users.FindStatusByIDForUpdate(txCtx, normalized.UserID)
		if findErr != nil {
			return findErr
		}
		var actionErr error
		record, actionErr = user.ApplyBan(normalized, time.Now())
		if actionErr != nil {
			return actionErr
		}
		if updateErr := s.users.SaveStatus(txCtx, user); updateErr != nil {
			return updateErr
		}

		if normalized.Status != enum.UserStatusActive {
			saved, createErr := s.records.Create(txCtx, record)
			if createErr != nil {
				return createErr
			}

			record = saved

			return nil
		}

		latest, findErr := s.records.FindLatestUnreleasedForUpdate(
			txCtx,
			user.ID(),
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

		latest.Release()
		if markErr := s.records.MarkUnbanned(txCtx, latest.ID()); markErr != nil {
			return markErr
		}

		releasedRecordID = latest.ID()

		return nil
	}); err != nil {
		return nil, err
	}

	// 提交后从最新事实做 generation/CAS 回填，避免较晚提交的封禁被旧动作覆盖。
	_ = s.statuses.Refresh(ctx, user.ID())
	s.notifyBanStatus(ctx, user, record)

	result := ToBanRecordDTO(record)
	// 解封事件使用对应封禁记录 ID，作为响应事实，不向新建领域记录赋予身份。
	if releasedRecordID != 0 {
		result.ID = releasedRecordID
	}
	return result, nil
}

func (s *BanService) notifyBanStatus(ctx context.Context, user *identitydomain.User, record *identitydomain.BanRecord) {
	if user == nil || user.ID() == 0 || record == nil || s.notifier == nil {
		return
	}

	content := record.Notice()

	var (
		actionUserID     uint
		actionUserName   string
		actionUserAvatar string
	)

	if record.OperatorID() > 0 {
		operator, err := s.users.FindProfileByID(ctx, record.OperatorID())
		if err == nil && operator != nil {
			actionUserID = operator.ID()
			actionUserName = operator.Username()
			actionUserAvatar = operator.Avatar()
		} else {
			actionUserID = record.OperatorID()
		}
	}

	s.notifier.NotifyBanStatus(ctx, user.ID(), actionUserID, actionUserName, actionUserAvatar, banNotifyTitle, content)
}

// ReleaseExpired 原子地解封到期用户并标记原封禁记录，返回本次是否完成解封。
func (s *BanService) ReleaseExpired(ctx context.Context, userID uint, now time.Time) (bool, error) {
	if userID == 0 {
		return false, identitydomain.ErrBanUserIDRequired
	}

	unbanned := false
	if err := s.txm.Do(ctx, func(txCtx context.Context) error {
		user, err := s.users.FindStatusByIDForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		record, findErr := s.records.FindLatestUnreleasedForUpdate(
			txCtx,
			userID,
			[]enum.UserStatus{enum.UserStatusBanned},
		)
		if findErr != nil {
			return findErr
		}

		if !user.ReleaseExpiredBan(record, now) {
			return nil
		}

		activated, activateErr := s.users.ActivateIfBanned(txCtx, userID)
		if activateErr != nil {
			return activateErr
		}

		if !activated {
			return nil
		}

		if markErr := s.records.MarkUnbanned(txCtx, record.ID()); markErr != nil {
			return markErr
		}

		unbanned = true

		return nil
	}); err != nil {
		return false, err
	}

	if unbanned {
		_ = s.statuses.Refresh(ctx, userID)
	}

	return unbanned, nil
}

// BanRecordService 封禁记录查询用例服务。
type BanRecordService struct {
	records identitydomain.BanRecordRepository
	query   BanRecordQuery
}

// NewBanRecordService 构造封禁记录查询服务。
func NewBanRecordService(records identitydomain.BanRecordRepository, query BanRecordQuery) *BanRecordService {
	return &BanRecordService{records: records, query: query}
}

// Latest 读取用户最近一条未解封封禁记录；无记录时返回 (nil, nil)。
func (s *BanRecordService) Latest(ctx context.Context, userID uint) (*identitydomain.BanRecord, error) {
	if userID == 0 {
		return nil, identitydomain.ErrBanUserIDRequired
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
		return identitydomain.ErrUserNotLogin
	}
	user, err := s.users.FindStatusByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return identitydomain.ErrUserNotFound
	}
	if user.HasTimedBan() {
		if _, err := s.ReleaseExpired(ctx, userID, time.Now()); err != nil {
			return err
		}
		user, err = s.users.FindStatusByID(ctx, userID)
		if err != nil {
			return err
		}
		if user == nil {
			return identitydomain.ErrUserNotFound
		}
	}
	return user.EnsureCanPublish()
}
