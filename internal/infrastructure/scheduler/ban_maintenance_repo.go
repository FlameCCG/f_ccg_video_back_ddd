package scheduler

import (
	"context"
	"fmt"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BanMaintenanceRepository 封禁维护仓储的 GORM 实现（写侧投影，直连 users / user_ban_records）。
type BanMaintenanceRepository struct {
	db *gorm.DB
}

var _ identitydomain.BanMaintenanceRepository = (*BanMaintenanceRepository)(nil)

// NewBanMaintenanceRepository 构造封禁维护仓储。
func NewBanMaintenanceRepository(db *gorm.DB) *BanMaintenanceRepository {
	return &BanMaintenanceRepository{db: db}
}

// expiredBanRow 到期封禁查询的扫描行。
type expiredBanRow struct {
	RecordID uint       `gorm:"column:record_id"`
	UserID   uint       `gorm:"column:user_id"`
	EndAt    *time.Time `gorm:"column:end_at"`
}

// ListExpiredBans 查询每个用户最新的已到期封禁记录，且要求用户仍处于封禁态。
//
// 子查询取每个 user_id 的 MAX(id)，再 JOIN users，
// 过滤 u.status 与 r.status 为封禁中、r.end_at 非空且已到期。
func (r *BanMaintenanceRepository) ListExpiredBans(ctx context.Context, now time.Time) ([]identitydomain.ExpiredBan, error) {
	db := gormx.DB(ctx, r.db)

	subQuery := db.Model(&po.UserBanRecord{}).
		Select("MAX(id)").
		Group("user_id")

	var rows []expiredBanRow
	if err := db.
		Table("user_ban_records as r").
		Select("r.id as record_id, r.user_id, r.end_at").
		Joins("JOIN users u ON u.id = r.user_id").
		Where("r.id IN (?)", subQuery).
		Where(
			"u.status = ? AND r.status = ? AND r.is_unbanned = ? AND r.end_at IS NOT NULL AND r.end_at <= ?",
			enum.UserStatusBanned,
			enum.UserStatusBanned,
			false,
			now,
		).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	expired := make([]identitydomain.ExpiredBan, 0, len(rows))
	for _, row := range rows {
		expired = append(expired, identitydomain.ExpiredBan{
			RecordID: row.RecordID,
			UserID:   row.UserID,
			EndAt:    row.EndAt,
		})
	}

	return expired, nil
}

// ActivateBannedUsers 将仍处于封禁态的指定用户批量置为正常状态。
func (r *BanMaintenanceRepository) LatestBansForUpdate(ctx context.Context, userIDs []uint) ([]identitydomain.BanReleaseState, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	db := gormx.DB(ctx, r.db)
	var users []po.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status").Where("id IN ?", userIDs).Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	// 锁定读与用户行锁采用相同顺序。反连接直接选最新记录，避免子查询的旧一致性快照。
	var records []po.UserBanRecord
	if err := db.Table("user_ban_records AS r").Select("r.*").
		Joins("LEFT JOIN user_ban_records AS newer ON newer.user_id = r.user_id AND newer.id > r.id").
		Where("r.user_id IN ? AND newer.id IS NULL", userIDs).
		Order("r.user_id ASC").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&records).Error; err != nil {
		return nil, err
	}
	byUser := make(map[uint]*identitydomain.BanRecord, len(records))
	for _, record := range records {
		byUser[record.UserID] = identitydomain.RestoreBanRecord(identitydomain.BanRecordSnapshot{
			ID: record.ID, UserID: record.UserID, Status: record.Status,
			EndAt: record.EndAt, IsUnbanned: record.IsUnbanned,
		})
	}
	states := make([]identitydomain.BanReleaseState, 0, len(users))
	for _, user := range users {
		states = append(states, identitydomain.BanReleaseState{
			User:   identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: user.ID, Status: user.Status}),
			Record: byUser[user.ID],
		})
	}
	return states, nil
}

func (r *BanMaintenanceRepository) ActivateBannedUsers(ctx context.Context, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}

	result := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id IN ? AND status = ?", userIDs, enum.UserStatusBanned).
		Update("status", enum.UserStatusActive)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != int64(len(userIDs)) {
		return fmt.Errorf("auto unban state changed: updated %d of %d users", result.RowsAffected, len(userIDs))
	}
	return nil
}

// MarkBanRecordsUnbanned 批量标记原封禁记录已解封。
func (r *BanMaintenanceRepository) MarkBanRecordsUnbanned(ctx context.Context, recordIDs []uint) error {
	if len(recordIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.UserBanRecord{}).
		Where("id IN ?", recordIDs).
		Update("is_unbanned", true).Error
}
