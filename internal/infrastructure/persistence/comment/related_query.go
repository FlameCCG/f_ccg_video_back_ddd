package comment

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	app "f_ccg_video_back_ddd/internal/application/comment"

	"gorm.io/gorm"
)

// allowedAtUsersSQL 互相关注（任一方向）的 @ 候选过滤（与原实现逐字一致）。
const allowedAtUsersSQL = `
SELECT focus_user_id AS user_id
FROM user_focus
WHERE user_id = ? AND focus_user_id IN ?
UNION
SELECT user_id AS user_id
FROM user_focus
WHERE focus_user_id = ? AND user_id IN ?
`

// RelatedQuery 关联对象读侧投影的 GORM 实现。
//
// 直读 users / videos / dynamics / user_focus 表：按 docs/architecture.md 第 4 节，
// 读侧投影允许直读其它上下文的表，实现放在消费方上下文的 infrastructure。
type RelatedQuery struct {
	db *gorm.DB
}

var _ app.RelatedQuery = (*RelatedQuery)(nil)

// NewRelatedQuery 构造关联对象读侧投影。
func NewRelatedQuery(db *gorm.DB) *RelatedQuery {
	return &RelatedQuery{db: db}
}

// UserProfile 读取 id/username/avatar；不存在时返回 (nil, nil)。
func (q *RelatedQuery) UserProfile(ctx context.Context, userID uint) (*app.UserProfile, error) {
	var row po.User
	if err := gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Select("id", "username", "avatar").
		Where("id = ?", userID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.UserProfile{ID: row.ID, Username: row.Username, Avatar: row.Avatar}, nil
}

// UserProfileByUsername 按用户名读取 id/username/avatar；不存在时返回 (nil, nil)。
func (q *RelatedQuery) UserProfileByUsername(
	ctx context.Context,
	username string,
) (*app.UserProfile, error) {
	var row po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username", "avatar").
		Where("username = ?", username).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.UserProfile{ID: row.ID, Username: row.Username, Avatar: row.Avatar}, nil
}

// UsernameByID 读取用户名；查询失败或用户不存在时返回空串（与原实现忽略错误一致）。
func (q *RelatedQuery) UsernameByID(ctx context.Context, userID uint) string {
	var username string

	_ = gormx.DB(ctx, q.db).
		Model(&po.User{}).
		Where("id = ?", userID).
		Pluck("username", &username).Error

	return username
}

// PublishedVideoBrief 读取已发布视频的 id/title/author_username；不存在时返回 (nil, nil)。
func (q *RelatedQuery) PublishedVideoBrief(
	ctx context.Context,
	videoID uint,
) (*app.VideoBrief, error) {
	var row po.Video
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "title", "author_username").
		Where("id = ? AND status = ?", videoID, enum.VideoStatusPublished).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.VideoBrief{ID: row.ID, Title: row.Title, AuthorUsername: row.AuthorUsername}, nil
}

// VideoBrief 读取视频 id/title（不限状态）；不存在时返回 (nil, nil)。
func (q *RelatedQuery) VideoBrief(ctx context.Context, videoID uint) (*app.VideoBrief, error) {
	var row po.Video
	if err := gormx.DB(ctx, q.db).
		Select("id", "title").
		Where("id = ?", videoID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.VideoBrief{ID: row.ID, Title: row.Title}, nil
}

// VideoAuthorRef 读取视频及其作者用户 ID；视频不存在时返回 (nil, nil)。
//
// 作者关联（videos.author_username → users.username）缺失时 AuthorUserID 为 0，
// 与原 isVideoAuthor 中 video.Author.ID 为零值的判定一致。
func (q *RelatedQuery) VideoAuthorRef(ctx context.Context, videoID uint) (*app.VideoAuthorRef, error) {
	var row po.Video
	if err := gormx.DB(ctx, q.db).
		Select("id", "author_username").
		Preload("Author", func(tx *gorm.DB) *gorm.DB {
			return tx.Select("id", "username")
		}).
		Where("id = ?", videoID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.VideoAuthorRef{VideoID: row.ID, AuthorUserID: row.Author.ID}, nil
}

// Brief 读取动态 id/user_id；不存在时返回 (nil, nil)。
func (q *RelatedQuery) Brief(ctx context.Context, dynamicID uint) (*app.Brief, error) {
	var row po.Dynamic
	if err := gormx.DB(ctx, q.db).
		Model(&po.Dynamic{}).
		Select("id", "user_id").
		Where("id = ?", dynamicID).
		Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return &app.Brief{ID: row.ID, UserID: row.UserID}, nil
}

// AllowedAtUserIDs 过滤出与操作者互相关注（任一方向）的候选 @ 用户；
// 查询失败时返回 nil（与原实现忽略错误一致）。
func (q *RelatedQuery) AllowedAtUserIDs(
	ctx context.Context,
	actorID uint,
	candidateIDs []uint,
) []uint {
	if actorID == 0 || len(candidateIDs) == 0 {
		return nil
	}

	var rows []struct {
		UserID uint `gorm:"column:user_id"`
	}
	if err := gormx.DB(ctx, q.db).Raw(
		allowedAtUsersSQL,
		actorID, candidateIDs, actorID, candidateIDs,
	).Scan(&rows).Error; err != nil {
		return nil
	}

	result := make([]uint, 0, len(rows))

	for _, row := range rows {
		if row.UserID > 0 {
			result = append(result, row.UserID)
		}
	}

	return result
}
