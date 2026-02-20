package chat

import (
	"context"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// FollowRelationQuery 提供关注关系的只读 SQL 投影。
//
// 查询直读 user_focus 表，关注关系写入由 social 上下文负责。db 为 nil 时返回 (false, nil)。
type FollowRelationQuery struct {
	db *gorm.DB
}

var _ app.FollowRelationQuery = (*FollowRelationQuery)(nil)

// NewFollowRelationQuery 构造关注关系查询。
func NewFollowRelationQuery(db *gorm.DB) *FollowRelationQuery {
	return &FollowRelationQuery{db: db}
}

// IsFollowing 判断 userID 是否关注了 targetUserID。
func (q *FollowRelationQuery) IsFollowing(ctx context.Context, userID, targetUserID uint) (bool, error) {
	if userID == 0 || targetUserID == 0 {
		return false, nil
	}

	if q == nil || q.db == nil {
		return false, nil
	}

	var row struct {
		UserID uint `gorm:"column:user_id"`
	}

	err := gormx.DB(ctx, q.db).
		Model(&po.UserFocus{}).
		Select("user_id").
		Where("user_id = ? AND focus_user_id = ?", userID, targetUserID).
		Limit(1).
		Take(&row).Error
	if err == nil {
		return true, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}

	return false, err
}
