// Package contentops 是内容运营上下文的持久化实现：
// 轮播图 GORM 仓储、跨上下文读侧投影（分区存在性、站点统计）与配置文件写回。
package contentops

import (
	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainBanner 把轮播图 PO 转为领域实体（字段逐一显式映射）。
func toDomainBanner(row *po.Banner) *domain.Banner {
	if row == nil {
		return nil
	}

	return &domain.Banner{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Cover:       row.Cover,
		Href:        row.Href,
		Show:        row.Show,
		Type:        row.Type,
		PartitionID: row.PartitionID,
	}
}

// toDomainBanners 批量转换轮播图。
//
// 入参为 nil 时返回 nil：原实现把 GORM 查到的 nil 切片直接序列化为响应体（"data": null），
// 这里保持一致，不擅自替换成空数组。
func toDomainBanners(rows []po.Banner) []domain.Banner {
	if rows == nil {
		return nil
	}

	out := make([]domain.Banner, 0, len(rows))
	for i := range rows {
		out = append(out, *toDomainBanner(&rows[i]))
	}

	return out
}

// toPOBanner 把轮播图领域实体转为 PO。
func toPOBanner(banner *domain.Banner) *po.Banner {
	if banner == nil {
		return nil
	}

	return &po.Banner{
		Model: po.Model{
			ID:        banner.ID,
			CreatedAt: banner.CreatedAt,
			UpdatedAt: banner.UpdatedAt,
		},
		Cover:       banner.Cover,
		Href:        banner.Href,
		Show:        banner.Show,
		Type:        banner.Type,
		PartitionID: banner.PartitionID,
	}
}

// toPOBannerChanges 把部分更新集合翻译为 GORM 更新列。
//
// 列名与原 api/banner_api/admin_update.go 拼装的 map 键逐字一致。
func toPOBannerChanges(changes domain.BannerChanges) map[string]any {
	updates := map[string]any{}
	if changes.Cover != nil {
		updates["cover"] = *changes.Cover
	}

	if changes.Href != nil {
		updates["href"] = *changes.Href
	}

	if changes.Show != nil {
		updates["show"] = *changes.Show
	}

	if changes.Type != nil {
		updates["type"] = *changes.Type
	}

	if changes.PartitionID != nil {
		updates["partition_id"] = *changes.PartitionID
	}

	return updates
}
