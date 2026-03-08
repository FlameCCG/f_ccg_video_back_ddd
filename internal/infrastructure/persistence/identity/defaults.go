package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// InitializeDefaults explicitly initializes the account aggregate inside its creation transaction.
func (r *UserRepository) InitializeDefaults(ctx context.Context, userID uint) error {
	db := gormx.DB(ctx, r.db)
	if err := db.Create(&po.UserConf{UserID: userID, BannerID: r.defaultUserBannerID()}).Error; err != nil {
		return err
	}
	return db.Create(&po.FavoriteFolder{UserID: userID, Name: "默认收藏夹", IsDefault: true}).Error
}
