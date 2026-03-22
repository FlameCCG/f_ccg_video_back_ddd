package identity

import (
	"context"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"

	"f_ccg_video_back_ddd/pkg/constants"
)

// IsAdminUser 按已分配角色判断管理员，与后台登录使用相同的管理员角色。
// 不依赖用户当前切换的角色、用户名或客户端提交的身份。
func IsAdminUser(ctx context.Context, roles UserRoleQuery, userID uint) (bool, error) {
	if roles == nil {
		return false, ErrRoleCheckFailed
	}
	items, err := roles.LoadRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	names := make([]string, len(items))
	for i, role := range items {
		names[i] = role.Name
	}
	return (identitydomain.RoleAccess{}).HasRole(names, constants.AdminUserRole), nil
}
