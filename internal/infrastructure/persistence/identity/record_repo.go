package identity

import (
	"context"
	"errors"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExpRecordRepository 经验流水仓储的 GORM 实现。
type ExpRecordRepository struct {
	db *gorm.DB
}

var _ domain.ExpRecordRepository = (*ExpRecordRepository)(nil)

// NewExpRecordRepository 构造经验流水仓储。
func NewExpRecordRepository(db *gorm.DB) *ExpRecordRepository {
	return &ExpRecordRepository{db: db}
}

// Create 写入经验流水；带幂等键且命中唯一索引时返回 domain.ErrDuplicateRecord。
func (r *ExpRecordRepository) Create(ctx context.Context, userID uint, delta int64, reason string, dedupKey *string) error {
	rec := po.UserExpRecord{
		UserID:   userID,
		Delta:    delta,
		Reason:   reason,
		DedupKey: dedupKey,
	}

	if err := gormx.DB(ctx, r.db).Create(&rec).Error; err != nil {
		if dedupKey != nil && gormx.IsDuplicateKeyError(err) {
			return domain.ErrDuplicateRecord
		}

		return err
	}

	return nil
}

// SumDailyGained 统计指定自然日内的正向经验总量（本地时区自然日）。
func (r *ExpRecordRepository) SumDailyGained(ctx context.Context, userID uint, now time.Time) (int64, error) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var gained int64
	if err := gormx.DB(ctx, r.db).
		Model(&po.UserExpRecord{}).
		Select("COALESCE(SUM(delta), 0)").
		Where("user_id = ? AND delta > 0 AND created_at >= ? AND created_at < ?", userID, dayStart, dayEnd).
		Scan(&gained).Error; err != nil {
		return 0, err
	}

	return gained, nil
}

// CoinRecordRepository 硬币流水仓储的 GORM 实现。
type CoinRecordRepository struct {
	db *gorm.DB
}

var _ domain.CoinRecordRepository = (*CoinRecordRepository)(nil)

// NewCoinRecordRepository 构造硬币流水仓储。
func NewCoinRecordRepository(db *gorm.DB) *CoinRecordRepository {
	return &CoinRecordRepository{db: db}
}

// Create 写入硬币流水；带幂等键且命中唯一索引时返回 domain.ErrDuplicateRecord。
func (r *CoinRecordRepository) Create(ctx context.Context, userID uint, delta float64, reason string, dedupKey *string) error {
	rec := po.UserCoinRecord{
		UserID:   userID,
		Delta:    delta,
		Reason:   reason,
		DedupKey: dedupKey,
	}

	if err := gormx.DB(ctx, r.db).Create(&rec).Error; err != nil {
		if dedupKey != nil && gormx.IsDuplicateKeyError(err) {
			return domain.ErrDuplicateRecord
		}

		return err
	}

	return nil
}

// BanRecordRepository 封禁记录仓储的 GORM 实现。
type BanRecordRepository struct {
	db *gorm.DB
}

var _ domain.BanRecordRepository = (*BanRecordRepository)(nil)

// NewBanRecordRepository 构造封禁记录仓储。
func NewBanRecordRepository(db *gorm.DB) *BanRecordRepository {
	return &BanRecordRepository{db: db}
}

// Create 写入封禁记录并回填自增 ID 与时间戳。
func (r *BanRecordRepository) Create(ctx context.Context, record *domain.BanRecord) error {
	row := toPOBanRecord(record)
	if err := gormx.DB(ctx, r.db).Create(row).Error; err != nil {
		return err
	}

	record.ID = row.ID
	record.CreatedAt = row.CreatedAt
	record.UpdatedAt = row.UpdatedAt

	return nil
}

// FindLatestByUserID 读取用户最近一条未解封封禁记录；无记录时返回 (nil, nil)。
func (r *BanRecordRepository) FindLatestByUserID(ctx context.Context, userID uint) (*domain.BanRecord, error) {
	var row po.UserBanRecord
	if err := gormx.DB(ctx, r.db).
		Where("user_id = ? AND is_unbanned = ?", userID, false).
		Order("created_at DESC, id DESC").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainBanRecord(&row), nil
}

// FindLatestUnreleasedForUpdate 锁定并读取指定状态中最近一条未解封记录。
func (r *BanRecordRepository) FindLatestUnreleasedForUpdate(
	ctx context.Context,
	userID uint,
	statuses []enum.UserStatus,
) (*domain.BanRecord, error) {
	var row po.UserBanRecord
	if err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status IN ? AND is_unbanned = ?", userID, statuses, false).
		Order("created_at DESC, id DESC").
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainBanRecord(&row), nil
}

// MarkUnbanned 标记原封禁记录已解封。
func (r *BanRecordRepository) MarkUnbanned(ctx context.Context, recordID uint) error {
	return gormx.DB(ctx, r.db).
		Model(&po.UserBanRecord{}).
		Where("id = ?", recordID).
		Update("is_unbanned", true).Error
}

// LoginRecordRepository 登录记录仓储的 GORM 实现。
type LoginRecordRepository struct {
	db *gorm.DB
}

var _ domain.LoginRecordRepository = (*LoginRecordRepository)(nil)

// NewLoginRecordRepository 构造登录记录仓储。
func NewLoginRecordRepository(db *gorm.DB) *LoginRecordRepository {
	return &LoginRecordRepository{db: db}
}

// Create 写入登录记录。
func (r *LoginRecordRepository) Create(ctx context.Context, userID uint, ip, addr, userAgent, loginType string) error {
	rec := po.UserLoginRecord{
		UserID:    userID,
		IP:        ip,
		Addr:      addr,
		UserAgent: userAgent,
		LoginType: loginType,
	}

	return gormx.DB(ctx, r.db).Create(&rec).Error
}

func (r *CoinRecordRepository) HasDedupKey(ctx context.Context, uid uint, key string) (bool, error) {
	var n int64
	err := gormx.DB(ctx, r.db).Model(&po.UserCoinRecord{}).Where("user_id = ? AND dedup_key = ?", uid, key).Count(&n).Error
	return n > 0, err
}
