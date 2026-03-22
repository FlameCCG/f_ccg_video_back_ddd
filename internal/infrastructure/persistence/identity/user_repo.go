package identity

import (
	"context"
	"errors"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository 用户仓储的 GORM 实现。
type UserRepository struct {
	db          *gorm.DB
	cfgProvider *infraconfig.Provider
}

// 编译期断言：满足领域侧仓储接口。
var _ identitydomain.UserRepository = (*UserRepository)(nil)

// NewUserRepository 构造用户仓储。
func NewUserRepository(db *gorm.DB, cfgProvider *infraconfig.Provider) *UserRepository {
	return &UserRepository{db: db, cfgProvider: cfgProvider}
}

func (r *UserRepository) defaultUserBannerID() uint {
	cfg := r.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return identitydomain.FallbackDefaultUserBannerID
	}

	return (identitydomain.BannerPolicy{}).ResolveDefaultID(cfg.Site.DefaultUserBannerID)
}

// FindByID 按主键读取用户；不存在时返回 (nil, nil)。
func (r *UserRepository) FindByID(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).First(&row, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// FindWithConfByID 一次 LEFT JOIN 读取用户及其一对一配置；不存在时返回 (nil, nil, nil)。
func (r *UserRepository) FindWithConfByID(
	ctx context.Context,
	userID uint,
) (*identitydomain.User, *identitydomain.UserConf, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Joins("UserConf").First(&row, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}

		return nil, nil, err
	}

	return toDomainUser(&row), toDomainUserConf(row.UserConf), nil
}

// FindStatusByID 只读 id/status/username；不存在时返回 (nil, nil)。
func (r *UserRepository) FindStatusByID(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Select("id", "status", "username").
		Where("id = ?", userID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// FindProfileByID 只读 id/username/avatar；不存在时返回 (nil, nil)。
func (r *UserRepository) FindProfileByID(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Select("id", "username", "avatar").
		Where("id = ?", userID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return toDomainUser(&row), nil
}

// UsernameByID 读取用户名；用户不存在时返回空串。
func (r *UserRepository) UsernameByID(ctx context.Context, userID uint) (string, error) {
	var username string
	if err := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Pluck("username", &username).Error; err != nil {
		return "", err
	}

	return username, nil
}

// SaveStatus 仅更新封禁动作涉及的状态列。
func (r *UserRepository) SaveStatus(ctx context.Context, user *identitydomain.User) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", user.ID()).
		Update("status", user.Status()).Error
}

func (r *UserRepository) FindByIDForUpdate(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&row, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(&row), nil
}

func (r *UserRepository) FindByEmailForUpdate(ctx context.Context, email string) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(&row), nil
}

func (r *UserRepository) FindStatusByIDForUpdate(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "status", "username").Take(&row, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainUser(&row), nil
}

// ActivateIfBanned 仅当用户仍处于限时封禁态时置为正常，返回是否更新成功。
func (r *UserRepository) ActivateIfBanned(ctx context.Context, userID uint) (bool, error) {
	result := gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ? AND status = ?", userID, enum.UserStatusBanned).
		Update("status", enum.UserStatusActive)
	if result.Error != nil {
		return false, result.Error
	}

	return result.RowsAffected > 0, nil
}

// ExpAndLevelForUpdate 以行锁读取经验账户（要求在事务内调用）。
func (r *UserRepository) ExpAndLevelForUpdate(ctx context.Context, userID uint) (*identitydomain.User, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "exp", "level").
		Take(&row, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	return identitydomain.RestoreUser(identitydomain.UserSnapshot{
		ID:    row.ID,
		Exp:   row.Exp,
		Level: row.Level,
	}), nil
}

// UpdateExpAndLevel 覆盖写经验与等级。
func (r *UserRepository) UpdateExpAndLevel(ctx context.Context, userID uint, exp int64, level int) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"exp":   exp,
			"level": level,
		}).Error
}

// UpdateLastLogin 更新最后登录 IP 与归属地。
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uint, ip, addr string) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"last_login_ip":   ip,
			"last_login_addr": addr,
		}).Error
}
