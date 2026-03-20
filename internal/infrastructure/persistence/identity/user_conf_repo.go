package identity

import (
	"context"
	"encoding/json"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/identity"
	identityentity "f_ccg_video_back_ddd/internal/domain/identity/entity"
	identityrepo "f_ccg_video_back_ddd/internal/domain/identity/repository"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// UserConfRepository 用户配置仓储的 GORM 实现（同时提供读模型查询）。
type UserConfRepository struct {
	db *gorm.DB
}

var (
	_ identityrepo.UserConfRepository = (*UserConfRepository)(nil)
	_ app.UserConfQuery               = (*UserConfRepository)(nil)
)

// NewUserConfRepository 构造用户配置仓储。
func NewUserConfRepository(db *gorm.DB) *UserConfRepository {
	return &UserConfRepository{db: db}
}

// FindByUserID 读取用户配置；不存在时返回 (nil, nil)。
func (r *UserConfRepository) FindByUserID(ctx context.Context, userID uint) (*identityentity.UserConf, error) {
	row, err := r.load(ctx, userID)
	if err != nil || row == nil {
		return nil, err
	}

	return toDomainUserConf(row), nil
}

// View 读取用户配置读模型；不存在时返回 (nil, nil)。
func (r *UserConfRepository) View(ctx context.Context, userID uint) (*app.UserConfView, error) {
	row, err := r.load(ctx, userID)
	if err != nil || row == nil {
		return nil, err
	}

	return toUserConfView(row), nil
}

func (r *UserConfRepository) load(ctx context.Context, userID uint) (*po.UserConf, error) {
	var row po.UserConf
	if err := gormx.DB(ctx, r.db).Where("user_id = ?", userID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &row, nil
}

// Update 部分更新用户配置。
func (r *UserConfRepository) Update(ctx context.Context, userID uint, changes identityentity.UserConfChanges) error {
	updates, err := userConfUpdateColumns(changes)
	if err != nil {
		return err
	}

	if len(updates) == 0 {
		return nil
	}

	return gormx.DB(ctx, r.db).
		Model(&po.UserConf{}).
		Where("user_id = ?", userID).
		Updates(updates).Error
}

// ReassignBanner 把所有使用 fromBannerID 的用户配置改为 toBannerID。
func (r *UserConfRepository) ReassignBanner(ctx context.Context, fromBannerID, toBannerID uint) error {
	return gormx.DB(ctx, r.db).
		Model(&po.UserConf{}).
		Where("banner_id = ?", fromBannerID).
		Update("banner_id", toBannerID).Error
}

// userConfUpdateColumns 把领域侧的部分更新集合翻译为列名 → 值。
func userConfUpdateColumns(changes identityentity.UserConfChanges) (map[string]any, error) {
	updates := make(map[string]any, 8)

	if changes.OpenCollect != nil {
		updates["open_collect"] = *changes.OpenCollect
	}

	if changes.OpenFans != nil {
		updates["open_fans"] = *changes.OpenFans
	}

	if changes.OpenFollow != nil {
		updates["open_follow"] = *changes.OpenFollow
	}

	if changes.OpenLikeVideo != nil {
		updates["open_like_video"] = *changes.OpenLikeVideo
	}

	if changes.OpenCoinVideo != nil {
		updates["open_coin_video"] = *changes.OpenCoinVideo
	}

	if changes.BannerID != nil {
		updates["banner_id"] = *changes.BannerID
	}

	if changes.BannerURL != nil {
		updates["banner_url"] = *changes.BannerURL
	}

	if changes.LikeTags != nil {
		// 显式返回 Marshal 错误，避免序列化失败时静默丢失字段。
		data, err := json.Marshal(*changes.LikeTags)
		if err != nil {
			return nil, err
		}

		updates["like_tags"] = string(data)
	}

	return updates, nil
}
