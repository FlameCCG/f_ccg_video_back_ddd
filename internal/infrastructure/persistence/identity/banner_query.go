package identity

import (
	"context"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// UserBannerQuery 用户主页横幅目录查询的 GORM 实现。
//
// banners 表归 content-ops 上下文所有，这里是读侧投影（见 decisions.md D-011）：
// SQL 与原 banner_service.ListVisibleByIDs / resolveUserBannerURL 完全一致。
type UserBannerQuery struct {
	db *gorm.DB
}

var _ app.UserBannerCatalog = (*UserBannerQuery)(nil)

// NewUserBannerQuery 构造用户横幅目录查询。
func NewUserBannerQuery(db *gorm.DB) *UserBannerQuery {
	return &UserBannerQuery{db: db}
}

// FindVisible 读取指定 ID 的可见用户横幅；不存在时返回 (nil, nil)。
func (q *UserBannerQuery) FindVisible(ctx context.Context, bannerID uint) (*app.UserBannerItem, error) {
	var row po.Banner
	if err := gormx.DB(ctx, q.db).
		Where("id = ? AND type = ? AND `show` = ?", bannerID, enum.BannerTypeUser, true).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	item := toUserBannerItem(row)

	return &item, nil
}

// ListVisible 按传入 ID 顺序读取可见用户横幅，缺失的跳过。
func (q *UserBannerQuery) ListVisible(ctx context.Context, bannerIDs []uint) ([]app.UserBannerItem, error) {
	if len(bannerIDs) == 0 {
		return []app.UserBannerItem{}, nil
	}

	rows := make([]po.Banner, 0, len(bannerIDs))
	if err := gormx.DB(ctx, q.db).
		Model(&po.Banner{}).
		Where("`show` = ?", true).
		Where("id IN ?", bannerIDs).
		Where("type = ?", enum.BannerTypeUser).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	bannerMap := make(map[uint]po.Banner, len(rows))
	for _, row := range rows {
		bannerMap[row.ID] = row
	}

	// 结果按请求的 ID 顺序返回（与原实现一致）。
	ordered := make([]app.UserBannerItem, 0, len(rows))

	for _, id := range bannerIDs {
		row, ok := bannerMap[id]
		if !ok {
			continue
		}

		ordered = append(ordered, toUserBannerItem(row))
	}

	return ordered, nil
}
