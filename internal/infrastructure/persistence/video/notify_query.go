package video

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// NotifySourceQuery 通知渲染所需的读侧投影查询的 GORM 实现。
//
// 按 decisions.md D-011：这些只读投影（users / videos 列）允许在本包直读；
// 写入通知行仍走 application 层的 VideoNotifier 端口。
type NotifySourceQuery struct {
	db *gorm.DB
}

var _ app.NotifySourceQuery = (*NotifySourceQuery)(nil)

// NewNotifySourceQuery 构造通知读侧投影查询。
func NewNotifySourceQuery(db *gorm.DB) *NotifySourceQuery {
	return &NotifySourceQuery{db: db}
}

// ActionUser 读取用户 id/username/avatar；不存在时返回 (零值, false)。
func (q *NotifySourceQuery) ActionUser(ctx context.Context, userID uint) (app.ActionUser, bool) {
	if userID == 0 {
		return app.ActionUser{}, false
	}

	var row po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username", "avatar").
		Where("id = ?", userID).
		First(&row).Error; err != nil {
		return app.ActionUser{}, false
	}

	return app.ActionUser{ID: row.ID, Username: row.Username, Avatar: row.Avatar}, true
}

// VideoAuthorAndTitle 读取视频标题与作者用户 ID；任一缺失时返回 ok=false。
func (q *NotifySourceQuery) VideoAuthorAndTitle(
	ctx context.Context,
	videoID uint,
) (uint, string, bool) {
	if videoID == 0 {
		return 0, "", false
	}

	db := gormx.DB(ctx, q.db)

	var video po.Video
	if err := db.
		Select("id", "title", "author_username").
		Where("id = ?", videoID).
		First(&video).Error; err != nil {
		return 0, "", false
	}

	var author po.User
	if err := db.
		Select("id").
		Where("username = ?", video.AuthorUsername).
		First(&author).Error; err != nil {
		return 0, "", false
	}

	return author.ID, video.Title, true
}

// UserIDsByUsernames 按用户名批量解析用户 ID。
func (q *NotifySourceQuery) UserIDsByUsernames(
	ctx context.Context,
	usernames []string,
) (map[string]uint, error) {
	if len(usernames) == 0 {
		return nil, nil
	}

	type userRow struct {
		ID       uint   `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}

	var users []userRow
	if err := gormx.DB(ctx, q.db).
		Table("users").
		Select("id, username").
		Where("username IN ?", usernames).
		Scan(&users).Error; err != nil {
		return nil, err
	}

	out := make(map[string]uint, len(users))

	for _, u := range users {
		if u.ID == 0 || u.Username == "" {
			continue
		}

		out[u.Username] = u.ID
	}

	return out, nil
}
