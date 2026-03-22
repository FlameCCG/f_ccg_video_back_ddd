// Package dynamic 实现动态写仓储、活动和关注流投影。
//
// dynamic_repo.go 保存动态及其关系，feed_query.go/activity_query.go/query.go 构建展示结果。
// 读侧联查用户、视频等表以减少用例拼接，不表示本上下文拥有这些表的业务写入规则。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package dynamic

import (
	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainDynamic 把动态 PO 转为领域实体。
func toDomainDynamic(row *po.Dynamic) *dynamicdomain.Dynamic {
	if row == nil {
		return nil
	}

	return dynamicdomain.RestoreDynamic(dynamicdomain.DynamicSnapshot{
		ID:        row.ID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
		UserID:    row.UserID,
		Content:   row.Content,
		ImageURL:  row.ImageURL,
		LikeCount: row.LikeCount,
		IsPinned:  row.IsPinned,
		PinnedAt:  row.PinnedAt,
	})
}

// toPODynamic 把动态领域实体转为 PO。
func toPODynamic(d *dynamicdomain.Dynamic) *po.Dynamic {
	if d == nil {
		return nil
	}

	return &po.Dynamic{
		Model: po.Model{
			ID:        d.ID(),
			CreatedAt: d.CreatedAt(),
			UpdatedAt: d.UpdatedAt(),
		},
		UserID:    d.UserID(),
		Content:   d.Content(),
		ImageURL:  d.ImageURL(),
		LikeCount: d.LikeCount(),
		IsPinned:  d.IsPinned(),
		PinnedAt:  d.PinnedAt(),
	}
}

// toDomainReadRecord 把已读位点 PO 转为领域实体。
func toDomainReadRecord(row *po.DynamicRead) *dynamicdomain.ReadRecord {
	if row == nil {
		return nil
	}

	return dynamicdomain.RestoreReadRecord(dynamicdomain.ReadRecordSnapshot{
		ID:               row.ID,
		UserID:           row.UserID,
		AuthorID:         row.AuthorID,
		LastReadWorkType: row.LastReadWorkType,
		LastReadWorkID:   row.LastReadWorkID,
		LastReadWorkAt:   row.LastReadWorkAt,
	})
}

// toPOReadRecord 把已读位点领域实体转为 PO。
func toPOReadRecord(record *dynamicdomain.ReadRecord) *po.DynamicRead {
	if record == nil {
		return nil
	}

	return &po.DynamicRead{
		Model: po.Model{
			ID: record.ID(),
		},
		UserID:           record.UserID(),
		AuthorID:         record.AuthorID(),
		LastReadWorkType: record.LastReadWorkType(),
		LastReadWorkID:   record.LastReadWorkID(),
		LastReadWorkAt:   record.LastReadWorkAt(),
	}
}
