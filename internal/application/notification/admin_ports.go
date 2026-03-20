package notification

import (
	"context"
)

// ActorProfileProvider 操作者资料查询端口（读侧投影，实现在 infrastructure）。
//
// 管理端发布系统通知时需要把操作者的用户名与头像写进通知快照字段。
type ActorProfileProvider interface {
	// Profile 返回操作者的用户名与头像；查不到时返回两个空串。
	Profile(ctx context.Context, userID uint) (username, avatar string)
}
