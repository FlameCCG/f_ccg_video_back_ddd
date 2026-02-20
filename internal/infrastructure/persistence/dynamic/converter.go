// Package dynamic 是动态上下文的持久化实现：GORM 仓储、读侧查询与 PO↔domain 转换。
package dynamic

import (
	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainDynamic 把动态 PO 转为领域实体。
func toDomainDynamic(row *po.Dynamic) *domain.Dynamic {
	if row == nil {
		return nil
	}

	return &domain.Dynamic{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		UserID:    row.UserID,
		Content:   row.Content,
		ImageURL:  row.ImageURL,
		LikeCount: row.LikeCount,
		IsPinned:  row.IsPinned,
		PinnedAt:  row.PinnedAt,
	}
}

// toPODynamic 把动态领域实体转为 PO。
func toPODynamic(d *domain.Dynamic) *po.Dynamic {
	if d == nil {
		return nil
	}

	return &po.Dynamic{
		Model: po.Model{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		},
		UserID:    d.UserID,
		Content:   d.Content,
		ImageURL:  d.ImageURL,
		LikeCount: d.LikeCount,
		IsPinned:  d.IsPinned,
		PinnedAt:  d.PinnedAt,
	}
}

// toDomainReadRecord 把已读位点 PO 转为领域实体。
func toDomainReadRecord(row *po.DynamicRead) *domain.ReadRecord {
	if row == nil {
		return nil
	}

	return &domain.ReadRecord{
		ID:               row.ID,
		UserID:           row.UserID,
		AuthorID:         row.AuthorID,
		LastReadWorkType: row.LastReadWorkType,
		LastReadWorkID:   row.LastReadWorkID,
		LastReadWorkAt:   row.LastReadWorkAt,
	}
}

// toPOReadRecord 把已读位点领域实体转为 PO。
func toPOReadRecord(record *domain.ReadRecord) *po.DynamicRead {
	if record == nil {
		return nil
	}

	return &po.DynamicRead{
		Model: po.Model{
			ID: record.ID,
		},
		UserID:           record.UserID,
		AuthorID:         record.AuthorID,
		LastReadWorkType: record.LastReadWorkType,
		LastReadWorkID:   record.LastReadWorkID,
		LastReadWorkAt:   record.LastReadWorkAt,
	}
}
