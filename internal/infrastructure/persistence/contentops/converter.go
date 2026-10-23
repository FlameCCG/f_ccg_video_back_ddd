// Package contentops 实现轮播图仓储、站点统计投影和配置文件写回。
//
// banner_repo.go/converter.go 负责轮播图持久化，query.go 读取运营所需的跨表事实。
// config_file_writer.go 写回 YAML 文件，它不与内存配置发布共享数据库事务。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
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
// 保持「空结果 = 非 nil 空切片」：GORM 的 Find 会把目标切片初始化为非 nil 空切片，
// 原实现把它直接序列化为响应体（"data": []），这里不改变该形态；
// 入参真为 nil（未经 Find 的路径）时同样返回 nil，避免凭空造出空数组。
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
