package identity

import (
	"context"
)

// AdminUserQuery 管理端用户列表查询端口（读侧，实现在 infrastructure）。
type AdminUserQuery interface {
	List(ctx context.Context, params AdminUserListParams) ([]AdminUserListItem, int64, error)
}
