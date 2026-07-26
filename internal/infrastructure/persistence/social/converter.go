// Package social 是社交关系上下文的持久化实现：GORM 仓储、读侧查询与 PO↔domain 转换。
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
