package search

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"

	app "f_ccg_video_back_ddd/internal/application/search"

	"gorm.io/gorm"
)

// MentionQuery 是 @ 提及候选集合查询（原 service/search_service/mention.go）。
//
// 候选只来自「当前用户关注的人」，并标注是否互关、带上被关注者的粉丝数。
type MentionQuery struct {
	db *gorm.DB
}

var _ app.MentionRelationQuery = (*MentionQuery)(nil)

// NewMentionQuery 构造 @ 提及候选查询（db 允许为 nil）。
func NewMentionQuery(db *gorm.DB) *MentionQuery {
	return &MentionQuery{db: db}
}

// mentionRelationRow @ 提及候选行。
type mentionRelationRow struct {
	ID            uint   `gorm:"column:id"`
	Username      string `gorm:"column:username"`
	Avatar        string `gorm:"column:avatar"`
	FollowerCount int64  `gorm:"column:follower_count"`
	IsFriend      bool   `gorm:"column:is_friend"`
}

// FollowingWithFriendFlag 返回 userID 关注的用户，并标注是否互关。
func (q *MentionQuery) FollowingWithFriendFlag(ctx context.Context, userID uint) ([]app.MentionUser, error) {
	if q == nil || q.db == nil {
		return nil, nil
	}

	var rows []mentionRelationRow
	if err := gormx.DB(ctx, q.db).Raw(`
		SELECT
			u.id,
			u.username,
			u.avatar,
			COALESCE(fc.cnt, 0) AS follower_count,
			CASE WHEN r.user_id IS NULL THEN 0 ELSE 1 END AS is_friend
		FROM user_focus f
		JOIN users u ON u.id = f.focus_user_id
		LEFT JOIN user_focus r ON r.user_id = f.focus_user_id AND r.focus_user_id = f.user_id
		LEFT JOIN (
			SELECT focus_user_id, COUNT(*) AS cnt
			FROM user_focus
			GROUP BY focus_user_id
		) fc ON fc.focus_user_id = u.id
		WHERE f.user_id = ?
	`, userID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	users := make([]app.MentionUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, app.MentionUser(row))
	}

	return users, nil
}
