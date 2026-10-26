package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"gorm.io/gorm"
)

// UserRelationQuery 关注关系与隐私开关查询的 GORM 实现。
//
// user_focus 归 social 上下文、dynamics 归 dynamic 上下文；这里是读侧投影
// （见 decisions.md D-011），SQL 与原 user_service/focus_service.go 逐字一致。
type UserRelationQuery struct {
	db *gorm.DB
}

var _ app.UserRelationQuery = (*UserRelationQuery)(nil)

// NewUserRelationQuery 构造关注关系查询。
func NewUserRelationQuery(db *gorm.DB) *UserRelationQuery {
	return &UserRelationQuery{db: db}
}

// RelationCounts 一次 SQL 取关注数/粉丝数（withDynamic 为 true 时附带动态数）。
//
// 与原实现一致：查询失败时返回零值而不是错误。
func (q *UserRelationQuery) RelationCounts(
	ctx context.Context,
	userID uint,
	withDynamic bool,
) app.UserRelationCounts {
	if userID == 0 {
		return app.UserRelationCounts{}
	}

	type row struct {
		FocusCount   int64 `gorm:"column:focus_count"`
		FansCount    int64 `gorm:"column:fans_count"`
		DynamicCount int64 `gorm:"column:dynamic_count"`
	}

	var data row

	query := `
SELECT
  (SELECT COUNT(*) FROM user_focus WHERE user_id = ?) AS focus_count,
  (SELECT COUNT(*) FROM user_focus WHERE focus_user_id = ?) AS fans_count,
  0 AS dynamic_count
`
	args := []any{userID, userID}

	if withDynamic {
		query = `
SELECT
  (SELECT COUNT(*) FROM user_focus WHERE user_id = ?) AS focus_count,
  (SELECT COUNT(*) FROM user_focus WHERE focus_user_id = ?) AS fans_count,
  (SELECT COUNT(*) FROM dynamics WHERE user_id = ?) AS dynamic_count
`

		args = append(args, userID)
	}

	if err := gormx.DB(ctx, q.db).Raw(query, args...).Scan(&data).Error; err != nil {
		return app.UserRelationCounts{}
	}

	return app.UserRelationCounts(data)
}

// DynamicCount 单独统计用户动态数；失败时返回 0（与原实现忽略错误一致）。
func (q *UserRelationQuery) DynamicCount(ctx context.Context, userID uint) int64 {
	var count int64

	_ = gormx.DB(ctx, q.db).
		Model(&po.Dynamic{}).
		Where("user_id = ?", userID).
		Count(&count).Error

	return count
}

// ActionPrivacy 读取最近点赞/投币是否公开（配置缺失时默认公开）。
func (q *UserRelationQuery) ActionPrivacy(ctx context.Context, userID uint) (app.UserActionPrivacy, error) {
	if userID == 0 {
		return app.UserActionPrivacy{}, domain.ErrUserIDRequired
	}

	type row struct {
		ID            uint `gorm:"column:id"`
		OpenLikeVideo int8 `gorm:"column:open_like_video"`
		OpenCoinVideo int8 `gorm:"column:open_coin_video"`
	}

	var data row
	if err := gormx.DB(ctx, q.db).
		Table("users").
		Select("users.id AS id, COALESCE(uc.open_like_video, 1) AS open_like_video, COALESCE(uc.open_coin_video, 1) AS open_coin_video").
		Joins("LEFT JOIN user_confs uc ON uc.user_id = users.id").
		Where("users.id = ?", userID).
		Take(&data).Error; err != nil {
		return app.UserActionPrivacy{}, err
	}

	return app.UserActionPrivacy{
		Exists:        data.ID > 0,
		OpenLikeVideo: data.OpenLikeVideo > 0,
		OpenCoinVideo: data.OpenCoinVideo > 0,
	}, nil
}
