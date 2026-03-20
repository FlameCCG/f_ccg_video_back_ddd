package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// BanRecordQuery 管理端封禁记录查询端口（读侧）。
type BanRecordQuery interface {
	AdminList(ctx context.Context, params AdminBanRecordListParams) ([]AdminBanRecordItem, int64, error)
}

// BanNotifier 封禁状态变更通知端口。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现，实现跨上下文协调。
type BanNotifier interface {
	// NotifyBanStatus 向被处理用户发送系统通知。
	NotifyBanStatus(ctx context.Context, receiverID uint, actionUserID uint, actionUserName, actionUserAvatar, title, content string)
}

// UserStatusReader 用户状态读取端口，供其它上下文（如中间件封禁校验）复用。
type UserStatusReader interface {
	CachedStatus(ctx context.Context, userID uint) (enum.UserStatus, error)
}
