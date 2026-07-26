package identity

import "context"

// AdminUserService 管理端用户查询用例服务（原 service/user_service/admin_list.go）。
type AdminUserService struct {
	query AdminUserQuery
}

// NewAdminUserService 构造管理端用户查询服务。
func NewAdminUserService(query AdminUserQuery) *AdminUserService {
	return &AdminUserService{query: query}
}

// List 管理端用户分页列表（支持关键词/状态/角色过滤）。
func (s *AdminUserService) List(ctx context.Context, params AdminUserListParams) ([]AdminUserListItem, int64, error) {
	return s.query.List(ctx, params)
}
