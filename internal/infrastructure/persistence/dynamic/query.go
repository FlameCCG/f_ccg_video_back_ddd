package dynamic

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"

	"gorm.io/gorm"
)

// AdminDynamicQuery 管理端动态列表查询的 GORM 实现。
type AdminDynamicQuery struct {
	db *gorm.DB
}

var _ app.AdminDynamicQuery = (*AdminDynamicQuery)(nil)

// NewAdminDynamicQuery 构造管理端动态列表查询。
func NewAdminDynamicQuery(db *gorm.DB) *AdminDynamicQuery {
	return &AdminDynamicQuery{db: db}
}

// List 管理端动态分页列表（原 service/dynamic_service/admin_service.go 的 AdminList）。
//
// 排序列显式带 dynamics 前缀：按用户名过滤时会 JOIN users，
// 原实现的裸列名在 MySQL 下会触发 1052「列名歧义」，这里通过显式表别名避免歧义。
func (q *AdminDynamicQuery) List(
	ctx context.Context,
	params app.AdminDynamicListParams,
) ([]app.Item, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Dynamic{})
	if params.UserID > 0 {
		query = query.Where("user_id = ?", params.UserID)
	}

	if params.Username != "" {
		query = query.Joins("JOIN users u ON u.id = dynamics.user_id").
			Where("u.username LIKE ?", "%"+params.Username+"%")
	}

	if params.Keyword != "" {
		query = query.Where("content LIKE ?", "%"+params.Keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []app.Item{}, 0, nil
	}

	var rows []po.Dynamic
	if err := query.
		Preload("User", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "username", "avatar")
		}).
		Order("dynamics.is_pinned DESC, dynamics.pinned_at DESC, dynamics.created_at DESC, dynamics.id DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]app.Item, 0, len(rows))
	for _, d := range rows {
		items = append(items, app.Item{
			ID:        d.ID,
			UserID:    d.UserID,
			Username:  d.User.Username,
			Avatar:    d.User.Avatar,
			Content:   d.Content,
			ImageURL:  d.ImageURL,
			IsPinned:  d.IsPinned,
			PinnedAt:  d.PinnedAt,
			CreatedAt: d.CreatedAt,
		})
	}

	return items, total, nil
}

// UserProfileQuery 用户资料读侧投影实现。
//
// 按 D-011，读侧投影允许直读 identity 上下文的 users 表；写侧仍归 identity。
type UserProfileQuery struct {
	db *gorm.DB
}

var _ app.UserProfileQuery = (*UserProfileQuery)(nil)

// NewUserProfileQuery 构造用户资料查询。
func NewUserProfileQuery(db *gorm.DB) *UserProfileQuery {
	return &UserProfileQuery{db: db}
}

// ProfileByID 读取 id/username/avatar；用户不存在时返回 (nil, nil)。
func (q *UserProfileQuery) ProfileByID(ctx context.Context, userID uint) (*app.UserProfile, error) {
	var row po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username", "avatar").
		Where("id = ?", userID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.UserProfile{ID: row.ID, Username: row.Username, Avatar: row.Avatar}, nil
}

// UsernameByID 读取用户名；用户不存在时返回空串（与原 Scan 语义一致）。
func (q *UserProfileQuery) UsernameByID(ctx context.Context, userID uint) (string, error) {
	var username string
	if err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Select("username").
		Where("id = ?", userID).
		Scan(&username).Error; err != nil {
		return "", err
	}

	return username, nil
}

// FollowRelationQuery 关注关系读侧投影实现。
//
// 关注关系写侧归 social 上下文；此处仅按 D-011 直读 user_focus 供关注流使用。
type FollowRelationQuery struct {
	db *gorm.DB
}

var _ app.FollowRelationQuery = (*FollowRelationQuery)(nil)

// NewFollowRelationQuery 构造关注关系查询。
func NewFollowRelationQuery(db *gorm.DB) *FollowRelationQuery {
	return &FollowRelationQuery{db: db}
}

// IsFollowing 判定 userID 是否关注了 authorID；查询失败按「未关注」处理（与原实现一致）。
func (q *FollowRelationQuery) IsFollowing(ctx context.Context, userID, authorID uint) bool {
	var count int64

	_ = gormx.DB(ctx, q.db).
		Table("user_focus").
		Where("user_id = ? AND focus_user_id = ?", userID, authorID).
		Limit(1).
		Count(&count).Error

	return count > 0
}

// ListFollowing 列出 userID 关注的全部用户（含用户名与头像）。
func (q *FollowRelationQuery) ListFollowing(ctx context.Context, userID uint) ([]app.FollowRelation, error) {
	type followRow struct {
		FocusUserID uint      `gorm:"column:focus_user_id"`
		FollowedAt  time.Time `gorm:"column:followed_at"`
		Username    string    `gorm:"column:username"`
		Avatar      string    `gorm:"column:avatar"`
	}

	var rows []followRow

	err := gormx.DB(ctx, q.db).
		Table("user_focus as uf").
		Select("uf.focus_user_id as focus_user_id, uf.created_at as followed_at, u.username as username, u.avatar as avatar").
		Joins("JOIN users u ON u.id = uf.focus_user_id").
		Where("uf.user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	relations := make([]app.FollowRelation, 0, len(rows))
	for _, r := range rows {
		relations = append(relations, app.FollowRelation{
			FocusUserID: r.FocusUserID,
			FollowedAt:  r.FollowedAt,
			Username:    r.Username,
			Avatar:      r.Avatar,
		})
	}

	return relations, nil
}

// VideoOwnershipQuery 视频归属读侧投影实现。
//
// 按 D-011 直读 video 上下文的 videos 表；视频置顶的写侧经 app.VideoPinWriter 端口。
type VideoOwnershipQuery struct {
	db *gorm.DB
}

var _ app.VideoOwnershipQuery = (*VideoOwnershipQuery)(nil)

// NewVideoOwnershipQuery 构造视频归属查询。
func NewVideoOwnershipQuery(db *gorm.DB) *VideoOwnershipQuery {
	return &VideoOwnershipQuery{db: db}
}

// ExistsPublishedByAuthor 判定视频是否存在、已发布且属于指定作者。
func (q *VideoOwnershipQuery) ExistsPublishedByAuthor(
	ctx context.Context,
	videoID uint,
	authorUsername string,
) (bool, error) {
	var exists int64
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("id = ? AND author_username = ? AND status = ?", videoID, authorUsername, enum.VideoStatusPublished).
		Limit(1).
		Count(&exists).Error; err != nil {
		return false, err
	}

	return exists > 0, nil
}
