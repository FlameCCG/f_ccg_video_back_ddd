package scheduler

import (
	"context"
	"time"

	app "f_ccg_video_back_ddd/internal/application/platform"
	domain "f_ccg_video_back_ddd/internal/domain/platform"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// BanMaintenanceRepository 封禁维护仓储的 GORM 实现（写侧投影，直连 users / user_ban_records）。
type BanMaintenanceRepository struct {
	db *gorm.DB
}

var _ app.BanMaintenanceRepository = (*BanMaintenanceRepository)(nil)

// NewBanMaintenanceRepository 构造封禁维护仓储。
func NewBanMaintenanceRepository(db *gorm.DB) *BanMaintenanceRepository {
	return &BanMaintenanceRepository{db: db}
}

// expiredBanRow 到期封禁查询的扫描行。
type expiredBanRow struct {
	UserID uint       `gorm:"column:user_id"`
	EndAt  *time.Time `gorm:"column:end_at"`
}

// ListExpiredBans 查询每个用户最新一条封禁记录中已到期、且用户仍处于封禁态的条目。
//
// SQL 与原实现逐字一致：子查询取每个 user_id 的 MAX(id)，
// 再 JOIN users 过滤 u.status / r.status 均为封禁中且 r.end_at 非空并已到期。
func (r *BanMaintenanceRepository) ListExpiredBans(ctx context.Context, now time.Time) ([]domain.ExpiredBan, error) {
	db := gormx.DB(ctx, r.db)

	subQuery := db.Model(&po.UserBanRecord{}).
		Select("MAX(id)").
		Group("user_id")

	var rows []expiredBanRow
	if err := db.
		Table("user_ban_records as r").
		Select("r.user_id, r.end_at").
		Joins("JOIN users u ON u.id = r.user_id").
		Where("r.id IN (?)", subQuery).
		Where(
			"u.status = ? AND r.status = ? AND r.end_at IS NOT NULL AND r.end_at <= ?",
			enum.UserStatusBanned, enum.UserStatusBanned, now,
		).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	expired := make([]domain.ExpiredBan, 0, len(rows))
	for _, row := range rows {
		expired = append(expired, domain.ExpiredBan{UserID: row.UserID, EndAt: row.EndAt})
	}

	return expired, nil
}

// ActivateBannedUsers 将仍处于封禁态的指定用户批量置为正常状态。
func (r *BanMaintenanceRepository) ActivateBannedUsers(ctx context.Context, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id IN ? AND status = ?", userIDs, enum.UserStatusBanned).
		Update("status", enum.UserStatusActive).Error
}

// CreateUnbanRecords 批量补写解封记录。
func (r *BanMaintenanceRepository) CreateUnbanRecords(ctx context.Context, records []domain.UnbanRecord) error {
	if len(records) == 0 {
		return nil
	}

	rows := make([]po.UserBanRecord, 0, len(records))
	for _, record := range records {
		rows = append(rows, po.UserBanRecord{
			UserID:     record.UserID,
			Status:     record.Status,
			Reason:     record.Reason,
			Days:       record.Days,
			StartAt:    record.StartAt,
			OperatorID: record.OperatorID,
		})
	}

	return gormx.DB(ctx, r.db).Create(&rows).Error
}
