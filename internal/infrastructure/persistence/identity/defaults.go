package identity

import (
	"context"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// InitializeDefaults 在创建事务内初始化身份配置；收藏夹由视频上下文用例负责。
func (r *UserRepository) InitializeDefaults(ctx context.Context, userID uint) error {
	db := gormx.DB(ctx, r.db)
	conf := identitydomain.NewUserConf(userID, r.defaultUserBannerID())
	return db.Create(&po.UserConf{
		UserID: conf.UserID(), BannerID: conf.BannerID(), OpenCollect: conf.OpenCollect(), OpenFans: conf.OpenFans(),
		OpenFollow: conf.OpenFollow(), OpenLikeVideo: conf.OpenLikeVideo(), OpenCoinVideo: conf.OpenCoinVideo(),
	}).Error
}
