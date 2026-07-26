// Package identity 是身份上下文的持久化实现：GORM 仓储与 PO↔domain 转换。
package identity

import (
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainUser 把用户 PO 转为领域实体。
func toDomainUser(row *po.User) *domain.User {
	if row == nil {
		return nil
	}

	return &domain.User{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Username:       row.Username,
		Email:          row.Email,
		Password:       row.Password,
		Avatar:         row.Avatar,
		Description:    row.Description,
		Gender:         row.Gender,
		Birthday:       row.Birthday,
		Level:          row.Level,
		OpenID:         row.OpenID,
		RegisterSource: row.RegisterSource,
		Exp:            row.Exp,
		CoinCount:      row.CoinCount,
		Status:         row.Status,
		CurrentRoleID:  row.CurrentRoleID,
		RegisterIP:     row.RegisterIP,
		LastLoginIP:    row.LastLoginIP,
		RegisterAddr:   row.RegisterAddr,
		LastLoginAddr:  row.LastLoginAddr,
	}
}

// toDomainBanRecord 把封禁记录 PO 转为领域实体。
func toDomainBanRecord(row *po.UserBanRecord) *domain.BanRecord {
	if row == nil {
		return nil
	}

	return &domain.BanRecord{
		ID:         row.ID,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		UserID:     row.UserID,
		Status:     row.Status,
		Reason:     row.Reason,
		Days:       row.Days,
		StartAt:    row.StartAt,
		EndAt:      row.EndAt,
		OperatorID: row.OperatorID,
	}
}

// toPOBanRecord 把封禁记录领域实体转为 PO。
func toPOBanRecord(record *domain.BanRecord) *po.UserBanRecord {
	if record == nil {
		return nil
	}

	return &po.UserBanRecord{
		Model: po.Model{
			ID:        record.ID,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
		},
		UserID:     record.UserID,
		Status:     record.Status,
		Reason:     record.Reason,
		Days:       record.Days,
		StartAt:    record.StartAt,
		EndAt:      record.EndAt,
		OperatorID: record.OperatorID,
	}
}
