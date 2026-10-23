package notification

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/notification"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// ActorQuery 操作者资料的读侧投影实现。
//
// 原 api/notification_api/create.go 在处理器里直接查 users 表取管理员的用户名与头像；
// 新架构把它收敛为应用层端口 + 读侧投影（按 docs/architecture.md 第 4 节，
// 读侧投影允许直读其它上下文的表；用户资料写侧仍归 identity）。
type ActorQuery struct {
	db *gorm.DB
}

var _ app.ActorProfileProvider = (*ActorQuery)(nil)

// NewActorQuery 构造操作者资料查询。
func NewActorQuery(db *gorm.DB) *ActorQuery {
	return &ActorQuery{db: db}
}

// Profile 读取操作者的用户名与头像；查询失败时返回两个空串（与原实现一致）。
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
