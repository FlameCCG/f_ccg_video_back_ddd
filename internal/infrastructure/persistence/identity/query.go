package identity

import (
	"context"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// recordListWindowDays 流水类列表默认只看最近 7 天（与原实现一致）。
const recordListWindowDays = -7

// AdminUserQuery 管理端用户列表查询的 GORM 实现。
type AdminUserQuery struct {
	db *gorm.DB
}

var _ app.AdminUserQuery = (*AdminUserQuery)(nil)

// NewAdminUserQuery 构造管理端用户列表查询。
func NewAdminUserQuery(db *gorm.DB) *AdminUserQuery {
	return &AdminUserQuery{db: db}
}

// List 管理端用户分页列表（原 service/user_service/admin_list.go）。
func (q *AdminUserQuery) List(
	ctx context.Context,
	params app.AdminUserListParams,
) ([]app.AdminUserListItem, int64, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := gormx.DB(ctx, q.db).Model(&po.User{})
	needDistinct := false

	if keyword := strings.TrimSpace(params.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR email LIKE ?", like, like)
	}

	if params.Status > 0 {
		query = query.Where("status = ?", params.Status)
	}

	if params.RoleID > 0 {
		query = query.Joins("JOIN user_roles ur ON ur.user_id = users.id").
			Where("ur.role_id = ?", params.RoleID)
		needDistinct = true
	}

	if needDistinct {
		query = query.Select("users.*").Distinct("users.id")
	}

	var total int64

	countQuery := query.Session(&gorm.Session{})
	if needDistinct {
		countQuery = countQuery.Distinct("users.id")
	}

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminUserListItem{}, 0, nil
	}

	var users []po.User
	if err := query.
		Preload("Roles").
		Order("created_at DESC, id DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminUserListItem, 0, len(users))

	for _, u := range users {
		item := app.AdminUserListItem{
			ID:             u.ID,
			Username:       u.Username,
			Email:          u.Email,
			Avatar:         u.Avatar,
			Status:         u.Status,
			Level:          u.Level,
			Exp:            u.Exp,
			CoinCount:      u.CoinCount,
			CurrentRoleID:  u.CurrentRoleID,
			RegisterSource: u.RegisterSource,
			CreatedAt:      u.CreatedAt,
		}

		if len(u.Roles) > 0 {
			roleNames := make([]string, 0, len(u.Roles))

			for _, role := range u.Roles {
				if role != nil && role.Name != "" {
					roleNames = append(roleNames, role.Name)
				}
			}

			item.RoleNames = roleNames
		}

		items = append(items, item)
	}

	return items, total, nil
}

// BanRecordQuery 管理端封禁记录查询的 GORM 实现。
type BanRecordQuery struct {
	db *gorm.DB
}

var _ app.BanRecordQuery = (*BanRecordQuery)(nil)

// NewBanRecordQuery 构造管理端封禁记录查询。
func NewBanRecordQuery(db *gorm.DB) *BanRecordQuery {
	return &BanRecordQuery{db: db}
}

// AdminList 管理端封禁记录分页列表（原 service/user_service/ban_record_service.go）。
func (q *BanRecordQuery) AdminList(
	ctx context.Context,
	params app.AdminBanRecordListParams,
) ([]app.AdminBanRecordItem, int64, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	params.Username = strings.TrimSpace(params.Username)

	query := gormx.DB(ctx, q.db).Table("user_ban_records as r").
		Joins("LEFT JOIN users u ON u.id = r.user_id").
		Joins("LEFT JOIN users op ON op.id = r.operator_id")

	if params.UserID > 0 {
		query = query.Where("r.user_id = ?", params.UserID)
	}

	if params.Username != "" {
		query = query.Where("u.username LIKE ?", "%"+params.Username+"%")
	}

	if params.Status > 0 {
		query = query.Where("r.status = ?", params.Status)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.AdminBanRecordItem{}, 0, nil
	}

	type row struct {
		ID               uint       `gorm:"column:id"`
		UserID           uint       `gorm:"column:user_id"`
		Username         string     `gorm:"column:username"`
		Status           int        `gorm:"column:status"`
		Reason           string     `gorm:"column:reason"`
		Days             int        `gorm:"column:days"`
		StartAt          time.Time  `gorm:"column:start_at"`
		EndAt            *time.Time `gorm:"column:end_at"`
		OperatorID       uint       `gorm:"column:operator_id"`
		OperatorUsername string     `gorm:"column:operator_username"`
		CreatedAt        time.Time  `gorm:"column:created_at"`
	}

	var rows []row
	if err := query.
		Select("r.id, r.user_id, r.status, r.reason, r.days, r.start_at, r.end_at, r.operator_id, r.created_at, u.username as username, op.username as operator_username").
		Order("r.created_at DESC, r.id DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.AdminBanRecordItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.AdminBanRecordItem{
			ID:               r.ID,
			UserID:           r.UserID,
			Username:         r.Username,
			Status:           enum.UserStatus(r.Status),
			Reason:           r.Reason,
			Days:             r.Days,
			StartAt:          r.StartAt,
			EndAt:            r.EndAt,
			OperatorID:       r.OperatorID,
			OperatorUsername: r.OperatorUsername,
			CreatedAt:        r.CreatedAt,
		})
	}

	return items, total, nil
}

// CoinRecordQuery 硬币流水查询的 GORM 实现。
type CoinRecordQuery struct {
	db *gorm.DB
}

var _ app.CoinRecordQuery = (*CoinRecordQuery)(nil)

// NewCoinRecordQuery 构造硬币流水查询。
func NewCoinRecordQuery(db *gorm.DB) *CoinRecordQuery {
	return &CoinRecordQuery{db: db}
}

// List 近 7 天硬币流水分页列表（原 service/user_service/coin_record_service.go）。
func (q *CoinRecordQuery) List(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]app.CoinRecordItem, int64, error) {
	page, pageSize = normalizeRecordPaging(page, pageSize)

	query := gormx.DB(ctx, q.db).Model(&po.UserCoinRecord{}).
		Where("user_id = ?", userID).
		Where("created_at >= ?", time.Now().AddDate(0, 0, recordListWindowDays))

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.CoinRecordItem{}, 0, nil
	}

	var rows []po.UserCoinRecord
	if err := query.
		Select("id", "delta", "reason", "created_at").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.CoinRecordItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.CoinRecordItem{
			ID:        r.ID,
			Delta:     r.Delta,
			Reason:    r.Reason,
			CreatedAt: r.CreatedAt,
		})
	}

	return items, total, nil
}

// ExpRecordQuery 经验流水查询的 GORM 实现。
type ExpRecordQuery struct {
	db *gorm.DB
}

var _ app.ExpRecordQuery = (*ExpRecordQuery)(nil)

// NewExpRecordQuery 构造经验流水查询。
func NewExpRecordQuery(db *gorm.DB) *ExpRecordQuery {
	return &ExpRecordQuery{db: db}
}

// List 近 7 天经验流水分页列表（原 service/user_service/exp_record_service.go）。
func (q *ExpRecordQuery) List(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]app.ExpRecordItem, int64, error) {
	page, pageSize = normalizeRecordPaging(page, pageSize)

	query := gormx.DB(ctx, q.db).Model(&po.UserExpRecord{}).Where("user_id = ?", userID)
	query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, recordListWindowDays))

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.ExpRecordItem{}, 0, nil
	}

	var rows []po.UserExpRecord
	if err := query.
		Select("id", "delta", "reason", "created_at").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.ExpRecordItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.ExpRecordItem{
			ID:        r.ID,
			Delta:     r.Delta,
			Reason:    r.Reason,
			CreatedAt: r.CreatedAt,
		})
	}

	return items, total, nil
}

// LoginRecordQuery 登录记录查询的 GORM 实现。
type LoginRecordQuery struct {
	db *gorm.DB
}

var _ app.LoginRecordQuery = (*LoginRecordQuery)(nil)

// NewLoginRecordQuery 构造登录记录查询。
func NewLoginRecordQuery(db *gorm.DB) *LoginRecordQuery {
	return &LoginRecordQuery{db: db}
}

// List 近 7 天登录记录分页列表（原 service/user_service/login_record_service.go）。
func (q *LoginRecordQuery) List(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]app.LoginRecordItem, int64, error) {
	page, pageSize = normalizeRecordPaging(page, pageSize)

	query := gormx.DB(ctx, q.db).Model(&po.UserLoginRecord{}).Where("user_id = ?", userID)
	query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, recordListWindowDays))

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.LoginRecordItem{}, 0, nil
	}

	var rows []po.UserLoginRecord
	if err := query.
		Select("id", "ip", "addr", "user_agent", "login_type", "created_at").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.LoginRecordItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.LoginRecordItem{
			ID:        r.ID,
			IP:        r.IP,
			Addr:      r.Addr,
			UserAgent: r.UserAgent,
			LoginType: r.LoginType,
			CreatedAt: r.CreatedAt,
		})
	}

	return items, total, nil
}

// normalizeRecordPaging 统一流水列表的分页参数纠偏（与原各服务实现一致）。
func normalizeRecordPaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}
