package notification

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// ActorQuery 提供操作者资料的只读投影。
//
// 查询直读 users 表获取用户名与头像，用户资料写入由 identity 上下文负责。
type ActorQuery struct {
	db *gorm.DB
}

var _ app.ActorProfileProvider = (*ActorQuery)(nil)

// NewActorQuery 构造操作者资料查询。
func NewActorQuery(db *gorm.DB) *ActorQuery {
	return &ActorQuery{db: db}
}

// Profile 读取操作者的用户名与头像；查询失败时返回两个空串。
func (q *ActorQuery) Profile(ctx context.Context, userID uint) (string, string) {
	if userID == 0 {
		return "", ""
	}

	var actor po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username", "avatar").
		Where("id = ?", userID).
		First(&actor).Error; err != nil {
		return "", ""
	}

	return actor.Username, actor.Avatar
}
