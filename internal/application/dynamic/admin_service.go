package dynamic

import (
	"context"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
)

// AdminService 管理端动态用例服务（原 service/dynamic_service/admin_service.go）。
type AdminService struct {
	dynamics domain.Repository
	txm      tx.Manager
	comments CommentPurger
	query    AdminDynamicQuery
}

// NewAdminService 构造管理端动态用例服务。
func NewAdminService(dynamics domain.Repository, query AdminDynamicQuery, txm tx.Manager, comments CommentPurger) *AdminService {
	return &AdminService{dynamics: dynamics, query: query, txm: txm, comments: comments}
}

// List 管理端动态分页列表（支持按用户 ID / 用户名 / 正文关键字过滤）。
func (s *AdminService) List(
	ctx context.Context,
	params AdminDynamicListParams,
) ([]Item, int64, error) {
	params.Page, params.PageSize = domain.NormalizeAdminPaging(params.Page, params.PageSize)
	params.Keyword = strings.TrimSpace(params.Keyword)
	params.Username = strings.TrimSpace(params.Username)

	return s.query.List(ctx, params)
}

// Delete 管理员批量删除动态，返回实际删除行数。
func (s *AdminService) Delete(ctx context.Context, dynamicIDs []uint) (int64, error) {
	if len(dynamicIDs) == 0 {
		return 0, domain.ErrDynamicIDsRequired
	}

	var affected int64
	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		for _, id := range dynamicIDs {
			if _, _, err := s.comments.PurgeByDynamic(txCtx, id); err != nil {
				return err
			}
		}
		var err error
		affected, err = s.dynamics.DeleteByIDs(txCtx, dynamicIDs)
		return err
	})
	return affected, err
}
