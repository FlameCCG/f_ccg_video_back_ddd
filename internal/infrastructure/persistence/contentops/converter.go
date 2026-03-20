// Package contentops 实现轮播图仓储、站点统计投影和配置文件写回。
//
// banner_repo.go/converter.go 负责轮播图持久化，query.go 读取运营所需的跨表事实。
// config_file_writer.go 写回 YAML 文件，它不与内存配置发布共享数据库事务。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package contentops

import (
	contentopsentity "f_ccg_video_back_ddd/internal/domain/contentops/entity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainBanner 把轮播图 PO 转为领域实体（字段逐一显式映射）。
func toDomainBanner(row *po.Banner) *contentopsentity.Banner {
	if row == nil {
		return nil
	}

	return &contentopsentity.Banner{
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
// GORM Find 的空结果是非 nil 空切片，响应体序列化为 data:[]。
// 未经 Find 的 nil 入参仍返回 nil，保留空值与空集合的区别。
func toDomainBanners(rows []po.Banner) []contentopsentity.Banner {
	if rows == nil {
		return nil
	}

	out := make([]contentopsentity.Banner, 0, len(rows))
	for i := range rows {
		out = append(out, *toDomainBanner(&rows[i]))
	}

	return out
}

// toPOBanner 把轮播图领域实体转为 PO。
func toPOBanner(banner *contentopsentity.Banner) *po.Banner {
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

// toPOBannerChanges 将部分更新集合转换为轮播图的 GORM 更新列。
func toPOBannerChanges(changes contentopsentity.BannerChanges) map[string]any {
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
