package search

import (
	"context"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"

	"gorm.io/gorm"
)

// UserDocumentQuery 装载写入 search_users 索引所需的用户数据（原 service/search_service/user_sync.go）。
//
// 每个用户额外统计：粉丝数、单向关注用户名、互关好友用户名——
// 这三个字段是 @ 提及联想与用户搜索的排序/召回依据，必须与 River 侧写入的字段保持一致。
type UserDocumentQuery struct {
	db *gorm.DB
}

var _ app.UserSearchDocumentQuery = (*UserDocumentQuery)(nil)

// NewUserDocumentQuery 构造用户搜索文档查询（db 允许为 nil）。
func NewUserDocumentQuery(db *gorm.DB) *UserDocumentQuery {
	return &UserDocumentQuery{db: db}
}

// userSyncRow 用户搜索文档的基础行。
type userSyncRow struct {
	ID          uint      `gorm:"column:id"`
	Username    string    `gorm:"column:username"`
	Description string    `gorm:"column:description"`
	Avatar      string    `gorm:"column:avatar"`
	Level       int       `gorm:"column:level"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

// LoadDocuments 按给定顺序装载用户文档；库中不存在的 ID 直接跳过。
func (q *UserDocumentQuery) LoadDocuments(ctx context.Context, userIDs []uint) ([]app.UserSearchDocument, error) {
	if q == nil || q.db == nil || len(userIDs) == 0 {
		return nil, nil
	}

	var rows []userSyncRow
	if err := gormx.DB(ctx, q.db).
		Table("users").
		Select("id, username, description, avatar, level, created_at, updated_at").
		Where("id IN ?", userIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	rowByID := make(map[uint]userSyncRow, len(rows))
	for _, row := range rows {
		rowByID[row.ID] = row
	}

	docs := make([]app.UserSearchDocument, 0, len(rows))

	for _, userID := range userIDs {
		row, ok := rowByID[userID]
		if !ok {
			continue
		}

		followerCount, followNames, friendNames, err := q.relationStats(ctx, row.ID)
		if err != nil {
			return nil, err
		}

		docs = append(docs, app.UserSearchDocument{
			ID:              row.ID,
			Username:        row.Username,
			Description:     row.Description,
			Avatar:          row.Avatar,
			Level:           row.Level,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
			FollowerCount:   followerCount,
			FollowUsernames: followNames,
			FriendUsernames: friendNames,
		})
	}

	return docs, nil
}

// relationStats 统计粉丝数、单向关注用户名、互关好友用户名。
func (q *UserDocumentQuery) relationStats(ctx context.Context, userID uint) (int64, []string, []string, error) {
	var followerCount int64
	if err := gormx.DB(ctx, q.db).
		Table("user_focus").
		Where("focus_user_id = ?", userID).
		Count(&followerCount).Error; err != nil {
		return 0, nil, nil, err
	}

	// 互相关注（好友）：当前用户关注对方，且对方也关注当前用户
	var friendNames []string
	if err := gormx.DB(ctx, q.db).Raw(`
		SELECT u.username
		FROM user_focus f
		JOIN user_focus r ON r.user_id = f.focus_user_id AND r.focus_user_id = f.user_id
		JOIN users u ON u.id = f.focus_user_id
		WHERE f.user_id = ?
		ORDER BY u.username ASC
	`, userID).Scan(&friendNames).Error; err != nil {
		return 0, nil, nil, err
	}

	// 单向关注（未互关）：当前用户关注的列表里排除已互关的
	var followNames []string
	if err := gormx.DB(ctx, q.db).Raw(`
		SELECT u.username
		FROM user_focus f
		JOIN users u ON u.id = f.focus_user_id
		WHERE f.user_id = ?
		  AND NOT EXISTS (
		    SELECT 1 FROM user_focus r WHERE r.user_id = f.focus_user_id AND r.focus_user_id = f.user_id
		  )
		ORDER BY u.username ASC
	`, userID).Scan(&followNames).Error; err != nil {
		return 0, nil, nil, err
	}

	return followerCount, followNames, friendNames, nil
}
