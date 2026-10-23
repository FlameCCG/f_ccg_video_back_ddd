// Package social 实现关注关系写入和关注/粉丝列表查询。
//
// focus_repo.go 实现关注写仓储，query.go 组合列表，converter.go 隔离持久化模型。
// 列表隐私由上层领域规则和用例判断；SQL 查询能力本身不等于允许向任意调用者展示。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package social

import (
	domain "f_ccg_video_back_ddd/internal/domain/social"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainFocus 把关注关系 PO 转为领域实体。
func toDomainFocus(row *po.UserFocus) *domain.Focus {
	if row == nil {
		return nil
	}

	return &domain.Focus{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		UserID:      row.UserID,
		FocusUserID: row.FocusUserID,
	}
}

// toPOFocus 把关注关系领域实体转为 PO。
func toPOFocus(focus *domain.Focus) *po.UserFocus {
	if focus == nil {
		return nil
	}

	return &po.UserFocus{
		Model: po.Model{
			ID:        focus.ID,
			CreatedAt: focus.CreatedAt,
			UpdatedAt: focus.UpdatedAt,
		},
		UserID:      focus.UserID,
		FocusUserID: focus.FocusUserID,
	}
}
