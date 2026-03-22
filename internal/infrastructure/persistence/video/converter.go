// Package video 实现视频写仓储、发布计划和页面查询投影。
//
// publish_repo.go 在单个 MySQL 事务中保存稿件、标签、分P、初始资源及发布时刻。
// video_repo.go/schedule_store.go 经 gormx 复用事务与行锁，避免改稿、审核和到期发布互相覆盖。
// *_query.go 直接构建应用读模型，converter.go 隔离 po 与领域实体；查询不必先重建整个写侧聚合。
// hard_delete_repo.go 返回待删对象，数据库提交不代表对象存储已清理；共享引用还需再次确认。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package video

import (
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainVideo 把视频 PO 转为领域实体。
func toDomainVideo(row *po.Video) *videodomain.Video {
	if row == nil {
		return nil
	}

	return videodomain.RestoreVideo(videodomain.VideoSnapshot{
		ID:             row.ID,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Title:          row.Title,
		Description:    row.Description,
		Cover:          row.Cover,
		Duration:       row.Duration,
		AuthorUsername: row.AuthorUsername,
		PartitionID:    row.PartitionID,
		Views:          row.Views,
		Likes:          row.Likes,
		CommentCount:   row.CommentCount,
		CoinCount:      row.CoinCount,
		FavoriteCount:  row.FavoriteCount,
		DanmuCount:     row.DanmuCount,
		IsOriginal:     row.IsOriginal,
		StorageType:    row.StorageType,
		Status:         row.Status,
		ApprovedAt:     row.ApprovedAt,
		IsScheduled:    row.IsScheduled,
	})
}

// toPOVideo 把视频领域实体转为 PO（仅投稿写入路径使用，不含关联）。
func toPOVideo(video *videodomain.Video) *po.Video {
	if video == nil {
		return nil
	}

	snap := video.Snapshot()

	return &po.Video{
		Model: po.Model{
			ID:        snap.ID,
			CreatedAt: snap.CreatedAt,
			UpdatedAt: snap.UpdatedAt,
		},
		Title:          snap.Title,
		Description:    snap.Description,
		Cover:          snap.Cover,
		Duration:       snap.Duration,
		AuthorUsername: snap.AuthorUsername,
		PartitionID:    snap.PartitionID,
		Views:          snap.Views,
		Likes:          snap.Likes,
		CommentCount:   snap.CommentCount,
		CoinCount:      snap.CoinCount,
		FavoriteCount:  snap.FavoriteCount,
		DanmuCount:     snap.DanmuCount,
		IsOriginal:     snap.IsOriginal,
		StorageType:    snap.StorageType,
		Status:         snap.Status,
		ApprovedAt:     snap.ApprovedAt,
		IsScheduled:    snap.IsScheduled,
	}
}

// toDomainStats 把视频 PO 的行内计数转为统计值对象。
func toDomainStats(row *po.Video) *videodomain.Stats {
	if row == nil {
		return &videodomain.Stats{}
	}

	return &videodomain.Stats{
		Views:         row.Views,
		Likes:         row.Likes,
		CommentCount:  row.CommentCount,
		CoinCount:     row.CoinCount,
		FavoriteCount: row.FavoriteCount,
		DanmuCount:    row.DanmuCount,
	}
}

// toDomainTag 把标签 PO 转为领域实体。
func toDomainTag(row *po.Tag) *videodomain.Tag {
	if row == nil {
		return nil
	}

	return &videodomain.Tag{ID: row.ID, Name: row.Name}
}

// toDomainTags 批量转换标签 PO。
func toDomainTags(rows []po.Tag) []videodomain.Tag {
	tags := make([]videodomain.Tag, 0, len(rows))
	for i := range rows {
		tags = append(tags, videodomain.Tag{ID: rows[i].ID, Name: rows[i].Name})
	}

	return tags
}

// toPOTags 批量把标签领域实体转为 PO（供 many2many 关联替换使用）。
func toPOTags(tags []videodomain.Tag) []po.Tag {
	rows := make([]po.Tag, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, po.Tag{Model: po.Model{ID: tag.ID}, Name: tag.Name})
	}

	return rows
}

// toDomainPartition 把分区 PO 转为领域实体。
func toDomainPartition(row *po.VideoPartition) *videodomain.Partition {
	if row == nil {
		return nil
	}

	return &videodomain.Partition{
		ID:            row.ID,
		Name:          row.Name,
		Icon:          row.Icon,
		SortOrder:     row.SortOrder,
		IsActive:      row.IsActive,
		IsSubmittable: row.IsSubmittable,
	}
}

// toPOPartition 把分区领域实体转为 PO。
func toPOPartition(p *videodomain.Partition) *po.VideoPartition {
	if p == nil {
		return nil
	}

	return &po.VideoPartition{
		Model:         po.Model{ID: p.ID},
		Name:          p.Name,
		Icon:          p.Icon,
		SortOrder:     p.SortOrder,
		IsActive:      p.IsActive,
		IsSubmittable: p.IsSubmittable,
	}
}

// toDomainPartitions 批量转换分区 PO。
func toDomainPartitions(rows []po.VideoPartition) []videodomain.Partition {
	partitions := make([]videodomain.Partition, 0, len(rows))
	for i := range rows {
		partitions = append(partitions, videodomain.Partition{
			ID:            rows[i].ID,
			Name:          rows[i].Name,
			Icon:          rows[i].Icon,
			SortOrder:     rows[i].SortOrder,
			IsActive:      rows[i].IsActive,
			IsSubmittable: rows[i].IsSubmittable,
		})
	}

	return partitions
}

// toDomainFolder 把收藏夹 PO 转为领域实体。
func toDomainFolder(row *po.FavoriteFolder) *videodomain.FavoriteFolder {
	if row == nil {
		return nil
	}

	return videodomain.RestoreFavoriteFolder(videodomain.FavoriteFolderSnapshot{
		ID:        row.ID,
		UserID:    row.UserID,
		Name:      row.Name,
		IsDefault: row.IsDefault,
	})
}

// toPOFolder 把收藏夹领域实体转为 PO。
func toPOFolder(folder *videodomain.FavoriteFolder) *po.FavoriteFolder {
	if folder == nil {
		return nil
	}

	snap := folder.Snapshot()

	return &po.FavoriteFolder{
		Model:     po.Model{ID: snap.ID},
		UserID:    snap.UserID,
		Name:      snap.Name,
		IsDefault: snap.IsDefault,
	}
}

// toDomainPlayHistory 把播放历史 PO 转为领域实体。
func toDomainPlayHistory(row *po.VideoPlayHistory) *videodomain.PlayHistory {
	if row == nil {
		return nil
	}

	return &videodomain.PlayHistory{
		UserID:    row.UserID,
		VideoID:   row.VideoID,
		Progress:  row.Progress,
		Duration:  row.Duration,
		CreatedAt: row.CreatedAt,
	}
}

// toDomainDanmu 把弹幕 PO 转为领域实体。
func toDomainDanmu(row *po.Danmu) *videodomain.Danmu {
	if row == nil {
		return nil
	}

	return videodomain.RestoreDanmu(videodomain.DanmuSnapshot{
		ID:          row.ID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		UserID:      row.UserID,
		VideoID:     row.VideoID,
		VideoPartID: row.VideoPartID,
		Content:     row.Content,
		TimeOffset:  row.TimeOffset,
		Color:       row.Color,
		FontSize:    row.FontSize,
		Position:    row.Position,
		LikeCount:   row.LikeCount,
	})
}

// toPODanmu 把弹幕领域实体转为 PO。
func toPODanmu(record *videodomain.Danmu) *po.Danmu {
	if record == nil {
		return nil
	}

	snap := record.Snapshot()

	return &po.Danmu{
		Model: po.Model{
			ID:        snap.ID,
			CreatedAt: snap.CreatedAt,
			UpdatedAt: snap.UpdatedAt,
		},
		UserID:      snap.UserID,
		VideoID:     snap.VideoID,
		VideoPartID: snap.VideoPartID,
		Content:     snap.Content,
		TimeOffset:  snap.TimeOffset,
		Color:       snap.Color,
		FontSize:    snap.FontSize,
		Position:    snap.Position,
		LikeCount:   snap.LikeCount,
	}
}

// toDomainDanmus 批量转换弹幕 PO。
func toDomainDanmus(rows []po.Danmu) []videodomain.Danmu {
	list := make([]videodomain.Danmu, 0, len(rows))
	for i := range rows {
		if converted := toDomainDanmu(&rows[i]); converted != nil {
			list = append(list, *converted)
		}
	}

	return list
}

// toPOVideoPart 把分P领域实体转为 PO。
func toPOVideoPart(part videodomain.VideoPart, videoID uint) po.VideoPart {
	return po.VideoPart{
		Model:       po.Model{ID: part.ID},
		VideoID:     videoID,
		Title:       part.Title,
		SortOrder:   part.SortOrder,
		Duration:    part.Duration,
		DanmuCount:  part.DanmuCount,
		StorageType: part.StorageType,
	}
}

// toPOVideoResource 把初始资源草稿转为视频资源 PO；draft 为 nil 时返回 nil。
func toPOVideoResource(draft *videodomain.ResourceDraft, videoID uint) *po.VideoResource {
	if draft == nil || videoID == 0 {
		return nil
	}

	return &po.VideoResource{
		VideoID:        videoID,
		Resolution:     draft.Resolution,
		SourceFileName: draft.SourceFileName,
		FileURL:        draft.FileURL,
		FileSize:       draft.FileSize,
		Bitrate:        draft.Bitrate,
		Format:         draft.Format,
		Codec:          draft.Codec,
		IsVip:          draft.IsVip,
		IsSouceVideo:   draft.IsSource,
	}
}

// toPOVideoPartResource 把初始资源草稿转为分P资源 PO；draft 为 nil 时返回 nil。
func toPOVideoPartResource(draft *videodomain.ResourceDraft, partID uint) *po.VideoPartResource {
	if draft == nil || partID == 0 {
		return nil
	}

	return &po.VideoPartResource{
		VideoPartID:    partID,
		Resolution:     draft.Resolution,
		SourceFileName: draft.SourceFileName,
		FileURL:        draft.FileURL,
		FileSize:       draft.FileSize,
		Bitrate:        draft.Bitrate,
		Format:         draft.Format,
		Codec:          draft.Codec,
		IsVip:          draft.IsVip,
		IsSouceVideo:   draft.IsSource,
	}
}
