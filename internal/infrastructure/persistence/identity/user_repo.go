package identity

import (
	"context"
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserRepository 用户仓储的 GORM 实现。
type UserRepository struct {
	db          *gorm.DB
	cfgProvider *config.Provider
}

// 编译期断言：满足领域侧仓储接口。
var _ domain.UserRepository = (*UserRepository)(nil)

// NewUserRepository 构造用户仓储。
//
// cfgProvider 用于把「注册默认横幅 ID」注入 context，供 po.User 的 AfterCreate 钩子读取
// （原实现由 models 层直接读全局配置，见 report.md FIND-002）。
func NewUserRepository(db *gorm.DB, cfgProvider *config.Provider) *UserRepository {
	return &UserRepository{db: db, cfgProvider: cfgProvider}
}

// WithDefaultBanner 在 context 上附加默认用户横幅 ID，供用户创建钩子使用。
func (r *UserRepository) WithDefaultBanner(ctx context.Context) context.Context {
	return po.WithDefaultUserBannerID(ctx, r.defaultUserBannerID())
}

func (r *UserRepository) defaultUserBannerID() uint {
	cfg := r.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil {
		return po.FallbackDefaultUserBannerID
	}

	return po.ResolveDefaultUserBannerID(cfg.Site.DefaultUserBannerID)
}

// FindByID 按主键读取用户；不存在时返回 (nil, nil)。
func (r *UserRepository) FindByID(ctx context.Context, userID uint) (*domain.User, error) {
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
) (*domain.User, *domain.UserConf, error) {
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
func (r *UserRepository) FindStatusByID(ctx context.Context, userID uint) (*domain.User, error) {
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
func (r *UserRepository) FindProfileByID(ctx context.Context, userID uint) (*domain.User, error) {
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

// UpdateStatus 更新用户状态。
func (r *UserRepository) UpdateStatus(ctx context.Context, userID uint, status enum.UserStatus) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Update("status", status).Error
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

// CoinBalanceForUpdate 以行锁读取硬币余额（要求在事务内调用）。
func (r *UserRepository) CoinBalanceForUpdate(ctx context.Context, userID uint) (float64, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "coin_count").
		Take(&row, "id = ?", userID).Error; err != nil {
		return 0, err
	}

	return row.CoinCount, nil
}

// CoinBalance 读取硬币余额（无锁）。
func (r *UserRepository) CoinBalance(ctx context.Context, userID uint) (float64, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Select("id", "coin_count").
		Take(&row, "id = ?", userID).Error; err != nil {
		return 0, err
	}

	return row.CoinCount, nil
}

// UpdateCoinBalance 覆盖写硬币余额。
func (r *UserRepository) UpdateCoinBalance(ctx context.Context, userID uint, coins float64) error {
	return gormx.DB(ctx, r.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Update("coin_count", coins).Error
}

// ExpAndLevelForUpdate 以行锁读取经验与等级（要求在事务内调用）。
func (r *UserRepository) ExpAndLevelForUpdate(ctx context.Context, userID uint) (int64, int, error) {
	var row po.User
	if err := gormx.DB(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "exp", "level").
		Take(&row, "id = ?", userID).Error; err != nil {
		return 0, 0, err
	}

	return row.Exp, row.Level, nil
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
