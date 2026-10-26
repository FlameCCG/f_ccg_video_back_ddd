package social

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/social"
	domain "f_ccg_video_back_ddd/internal/domain/social"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// 关注关系计数的原生 SQL（子查询写法与原 GetRelationCounts 逐字一致）。
const (
	relationCountsSQL = `
SELECT
  (SELECT COUNT(*) FROM user_focus WHERE user_id = ?) AS focus_count,
  (SELECT COUNT(*) FROM user_focus WHERE focus_user_id = ?) AS fans_count,
  0 AS dynamic_count
`

	relationCountsWithDynamicSQL = `
SELECT
  (SELECT COUNT(*) FROM user_focus WHERE user_id = ?) AS focus_count,
  (SELECT COUNT(*) FROM user_focus WHERE focus_user_id = ?) AS fans_count,
  (SELECT COUNT(*) FROM dynamics WHERE user_id = ?) AS dynamic_count
`

	// focusRelationSQL 一次往返取回双向关注关系（EXISTS 子查询命中唯一索引即可短路）。
	focusRelationSQL = `
SELECT
  EXISTS(SELECT 1 FROM user_focus WHERE user_id = ? AND focus_user_id = ?) AS is_focus,
  EXISTS(SELECT 1 FROM user_focus WHERE user_id = ? AND focus_user_id = ?) AS is_fans
`
)

// RelationQuery 关注关系读侧查询的 GORM 实现。
//
// users / user_confs / dynamics 分属 identity 与 dynamic 上下文，
// 此处按 D-011 仅做读侧投影。
type RelationQuery struct {
	db *gorm.DB
}

var _ app.RelationQuery = (*RelationQuery)(nil)

// NewRelationQuery 构造关注关系查询。
func NewRelationQuery(db *gorm.DB) *RelationQuery {
	return &RelationQuery{db: db}
}

// Exists 判定 userID 是否关注了 targetUserID（原 UserService.IsFocus）。
func (q *RelationQuery) Exists(ctx context.Context, userID, targetUserID uint) bool {
	var row struct {
		UserID uint `gorm:"column:user_id"`
	}

	err := gormx.DB(ctx, q.db).
		Model(&po.UserFocus{}).
		Select("user_id").
		Where("user_id = ? AND focus_user_id = ?", userID, targetUserID).
		Limit(1).
		Take(&row).Error

	return err == nil
}

// Relation 一次查询取回双向关注关系；查询失败按「无关系」处理（与原实现一致）。
func (q *RelationQuery) Relation(ctx context.Context, userID, targetUserID uint) domain.Relation {
	if userID == 0 || targetUserID == 0 {
		return domain.Relation{}
	}

	type row struct {
		IsFocus int8 `gorm:"column:is_focus"`
		IsFans  int8 `gorm:"column:is_fans"`
	}

	var data row
	if err := gormx.DB(ctx, q.db).
		Raw(focusRelationSQL, userID, targetUserID, targetUserID, userID).
		Scan(&data).Error; err != nil {
		return domain.Relation{}
	}

	return domain.Relation{IsFocus: data.IsFocus > 0, IsFans: data.IsFans > 0}
}

// Counts 统计关注数/粉丝数（withDynamic 为真时附带动态数）；查询失败返回零值。
func (q *RelationQuery) Counts(ctx context.Context, userID uint, withDynamic bool) domain.RelationCounts {
	if userID == 0 {
		return domain.RelationCounts{}
	}

	type row struct {
		FocusCount   int64 `gorm:"column:focus_count"`
		FansCount    int64 `gorm:"column:fans_count"`
		DynamicCount int64 `gorm:"column:dynamic_count"`
	}

	var data row

	query := relationCountsSQL
	args := []any{userID, userID}

	if withDynamic {
		query = relationCountsWithDynamicSQL

		args = append(args, userID)
	}

	if err := gormx.DB(ctx, q.db).Raw(query, args...).Scan(&data).Error; err != nil {
		return domain.RelationCounts{}
	}

	return domain.RelationCounts{
		FocusCount:   data.FocusCount,
		FansCount:    data.FansCount,
		DynamicCount: data.DynamicCount,
	}
}

// FollowPrivacy 读取关注/粉丝列表公开性；无 user_confs 行时用 COALESCE 缺省为公开。
func (q *RelationQuery) FollowPrivacy(ctx context.Context, userID uint) (domain.FollowPrivacy, error) {
	data, err := q.privacySwitches(ctx, userID, "users.id AS id, COALESCE(uc.open_follow, 1) AS first_switch, COALESCE(uc.open_fans, 1) AS second_switch")
	if err != nil {
		return domain.FollowPrivacy{}, err
	}

	return domain.FollowPrivacy{Exists: data.ID > 0, OpenFollow: data.FirstSwitch > 0, OpenFans: data.SecondSwitch > 0}, nil
}

// ActionPrivacy 读取最近点赞/投币公开性；无 user_confs 行时用 COALESCE 缺省为公开。
func (q *RelationQuery) ActionPrivacy(ctx context.Context, userID uint) (domain.ActionPrivacy, error) {
	data, err := q.privacySwitches(ctx, userID, "users.id AS id, COALESCE(uc.open_like_video, 1) AS first_switch, COALESCE(uc.open_coin_video, 1) AS second_switch")
	if err != nil {
		return domain.ActionPrivacy{}, err
	}

	return domain.ActionPrivacy{Exists: data.ID > 0, OpenLikeVideo: data.FirstSwitch > 0, OpenCoinVideo: data.SecondSwitch > 0}, nil
}

type userPrivacySwitches struct {
	ID           uint
	FirstSwitch  int8
	SecondSwitch int8
}

// privacySwitches uses only the fixed projections declared by the privacy query methods.
func (q *RelationQuery) privacySwitches(ctx context.Context, userID uint, projection string) (userPrivacySwitches, error) {
	var data userPrivacySwitches

	err := gormx.DB(ctx, q.db).Table("users").Select(projection).Joins("LEFT JOIN user_confs uc ON uc.user_id = users.id").Where("users.id = ?", userID).Take(&data).Error

	return data, err
}

// ListFocus 分页列出 userID 关注的用户（原 ListFocusUsers）。
func (q *RelationQuery) ListFocus(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]app.ListUser, int64, error) {
	return q.listSocialUsers(ctx, userID, page, pageSize, keyword, true)
}

// ListFans 分页列出关注了 userID 的用户（原 ListFansUsers）。
func (q *RelationQuery) ListFans(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]app.ListUser, int64, error) {
	return q.listSocialUsers(ctx, userID, page, pageSize, keyword, false)
}

// socialListRow 关注/粉丝列表的扫描行。
type socialListRow struct {
	ID          uint   `gorm:"column:id"`
	Username    string `gorm:"column:username"`
	Avatar      string `gorm:"column:avatar"`
	Description string `gorm:"column:description"`
	Level       int    `gorm:"column:level"`
}

// listSocialUsers 关注/粉丝列表共用查询：计数与取行并发执行（原 listSocialUsers）。
func (q *RelationQuery) listSocialUsers(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
	listFocus bool,
) ([]app.ListUser, int64, error) {
	if ctx == nil {
		ctx = context.TODO()
	}

	buildBase := func() *gorm.DB {
		db := gormx.DB(ctx, q.db).Table("user_focus")
		if listFocus {
			db = db.Joins("JOIN users u ON u.id = user_focus.focus_user_id").
				Where("user_focus.user_id = ?", userID)
		} else {
			db = db.Joins("JOIN users u ON u.id = user_focus.user_id").
				Where("user_focus.focus_user_id = ?", userID)
		}

		if keyword != "" {
			db = db.Where("u.username LIKE ?", "%"+keyword+"%")
		}

		return db
	}

	var (
		total int64
		rows  []socialListRow
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return buildBase().WithContext(groupCtx).Count(&total).Error
	})
	group.Go(func() error {
		query := buildBase().WithContext(groupCtx).
			Select("u.id, u.username, u.avatar, u.description, u.level").
			Order("user_focus.created_at DESC, u.id DESC").
			Offset((page - 1) * pageSize).
			Limit(pageSize)

		return query.Scan(&rows).Error
	})

	if err := group.Wait(); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.ListUser{}, 0, nil
	}

	users := make([]app.ListUser, 0, len(rows))
	for _, r := range rows {
		users = append(users, app.ListUser{
			ID:          r.ID,
			Username:    r.Username,
			Avatar:      r.Avatar,
			Description: r.Description,
			Level:       r.Level,
		})
	}

	return users, total, nil
}

// UserExistsQuery 目标用户存在性读侧投影实现（直读 identity 上下文的 users 表）。
type UserExistsQuery struct {
	db *gorm.DB
}

var _ app.UserExistsQuery = (*UserExistsQuery)(nil)

// NewUserExistsQuery 构造用户存在性查询。
func NewUserExistsQuery(db *gorm.DB) *UserExistsQuery {
	return &UserExistsQuery{db: db}
}

// Exists 判定用户是否存在（列选择与原 focus_user.go 的 Take 一致）。
func (q *UserExistsQuery) Exists(ctx context.Context, userID uint) (bool, error) {
	var target struct {
		ID uint
	}

	if err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Select("id").
		Where("id = ?", userID).
		Limit(1).
		Take(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
