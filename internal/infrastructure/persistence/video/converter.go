// Package video 是视频上下文的持久化实现：GORM 仓储、读侧查询与 PO↔domain 转换。
package video

import (
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// toDomainVideo 把视频 PO 转为领域实体。
func toDomainVideo(row *po.Video) *domain.Video {
	if row == nil {
		return nil
	}

	return &domain.Video{
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
	}
}

// toPOVideo 把视频领域实体转为 PO（仅投稿写入路径使用，不含关联）。
func toPOVideo(video *domain.Video) *po.Video {
	if video == nil {
		return nil
	}

	return &po.Video{
		Model: po.Model{
			ID:        video.ID,
			CreatedAt: video.CreatedAt,
			UpdatedAt: video.UpdatedAt,
		},
		Title:          video.Title,
		Description:    video.Description,
		Cover:          video.Cover,
		Duration:       video.Duration,
		AuthorUsername: video.AuthorUsername,
		PartitionID:    video.PartitionID,
		Views:          video.Views,
		Likes:          video.Likes,
		CommentCount:   video.CommentCount,
		CoinCount:      video.CoinCount,
		FavoriteCount:  video.FavoriteCount,
		DanmuCount:     video.DanmuCount,
		IsOriginal:     video.IsOriginal,
		StorageType:    video.StorageType,
		Status:         video.Status,
		ApprovedAt:     video.ApprovedAt,
		IsScheduled:    video.IsScheduled,
	}
}

// toDomainStats 把视频 PO 的行内计数转为统计值对象。
func toDomainStats(row *po.Video) *domain.Stats {
	if row == nil {
		return &domain.Stats{}
	}

	return &domain.Stats{
		Views:         row.Views,
		Likes:         row.Likes,
		CommentCount:  row.CommentCount,
		CoinCount:     row.CoinCount,
		FavoriteCount: row.FavoriteCount,
		DanmuCount:    row.DanmuCount,
	}
}

// toDomainTag 把标签 PO 转为领域实体。
func toDomainTag(row *po.Tag) *domain.Tag {
	if row == nil {
		return nil
	}

	return &domain.Tag{ID: row.ID, Name: row.Name}
}

// toDomainTags 批量转换标签 PO。
func toDomainTags(rows []po.Tag) []domain.Tag {
	tags := make([]domain.Tag, 0, len(rows))
	for i := range rows {
		tags = append(tags, domain.Tag{ID: rows[i].ID, Name: rows[i].Name})
	}

	return tags
}

// toPOTags 批量把标签领域实体转为 PO（供 many2many 关联替换使用）。
func toPOTags(tags []domain.Tag) []po.Tag {
	rows := make([]po.Tag, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, po.Tag{Model: po.Model{ID: tag.ID}, Name: tag.Name})
	}

	return rows
}

// toDomainPartition 把分区 PO 转为领域实体。
func toDomainPartition(row *po.VideoPartition) *domain.Partition {
	if row == nil {
		return nil
	}

	return &domain.Partition{
		ID:            row.ID,
		Name:          row.Name,
		Icon:          row.Icon,
		SortOrder:     row.SortOrder,
		IsActive:      row.IsActive,
		IsSubmittable: row.IsSubmittable,
	}
}

// toPOPartition 把分区领域实体转为 PO。
func toPOPartition(p *domain.Partition) *po.VideoPartition {
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
func toDomainPartitions(rows []po.VideoPartition) []domain.Partition {
	partitions := make([]domain.Partition, 0, len(rows))
	for i := range rows {
		partitions = append(partitions, domain.Partition{
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
func toDomainFolder(row *po.FavoriteFolder) *domain.FavoriteFolder {
	if row == nil {
		return nil
	}

	return &domain.FavoriteFolder{
		ID:        row.ID,
		UserID:    row.UserID,
		Name:      row.Name,
		IsDefault: row.IsDefault,
	}
}

// toPOFolder 把收藏夹领域实体转为 PO。
func toPOFolder(folder *domain.FavoriteFolder) *po.FavoriteFolder {
	if folder == nil {
		return nil
	}

	return &po.FavoriteFolder{
		Model:     po.Model{ID: folder.ID},
		UserID:    folder.UserID,
		Name:      folder.Name,
		IsDefault: folder.IsDefault,
	}
}

// toDomainPlayHistory 把播放历史 PO 转为领域实体。
func toDomainPlayHistory(row *po.VideoPlayHistory) *domain.PlayHistory {
	if row == nil {
		return nil
	}

	return &domain.PlayHistory{
		UserID:    row.UserID,
		VideoID:   row.VideoID,
		Progress:  row.Progress,
		Duration:  row.Duration,
		CreatedAt: row.CreatedAt,
	}
}

// toDomainDanmu 把弹幕 PO 转为领域实体。
func toDomainDanmu(row *po.Danmu) *domain.Danmu {
	if row == nil {
		return nil
	}

	return &domain.Danmu{
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
	}
}

// toPODanmu 把弹幕领域实体转为 PO。
func toPODanmu(record *domain.Danmu) *po.Danmu {
	if record == nil {
		return nil
	}

	return &po.Danmu{
		Model: po.Model{
			ID:        record.ID,
			CreatedAt: record.CreatedAt,
			UpdatedAt: record.UpdatedAt,
		},
		UserID:      record.UserID,
		VideoID:     record.VideoID,
		VideoPartID: record.VideoPartID,
		Content:     record.Content,
		TimeOffset:  record.TimeOffset,
		Color:       record.Color,
		FontSize:    record.FontSize,
		Position:    record.Position,
		LikeCount:   record.LikeCount,
	}
}

// toDomainDanmus 批量转换弹幕 PO。
func toDomainDanmus(rows []po.Danmu) []domain.Danmu {
	list := make([]domain.Danmu, 0, len(rows))
	for i := range rows {
		if converted := toDomainDanmu(&rows[i]); converted != nil {
			list = append(list, *converted)
		}
	}

	return list
}

// toDomainVideoReport 把视频举报 PO 转为领域实体（图片列表由调用方解析后传入）。
func toDomainVideoReport(row *po.VideoReport, imageURLs []string) *domain.VideoReport {
	if row == nil {
		return nil
	}

	return &domain.VideoReport{
		ID:               row.ID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		ReporterUsername: row.ReporterUsername,
		VideoID:          row.VideoID,
		Reason:           row.Reason,
		Detail:           row.Detail,
		ImageURLs:        imageURLs,
		Status:           row.Status,
		HandleNote:       row.HandleNote,
		HandledBy:        row.HandledBy,
		HandledAt:        row.HandledAt,
	}
}

// toDomainDanmuReport 把弹幕举报 PO 转为领域实体。
func toDomainDanmuReport(row *po.DanmuReport) *domain.DanmuReport {
	if row == nil {
		return nil
	}

	return &domain.DanmuReport{
		ID:               row.ID,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		ReporterUsername: row.ReporterUsername,
		VideoID:          row.VideoID,
		DanmuID:          row.DanmuID,
		Reason:           row.Reason,
		Detail:           row.Detail,
		Status:           row.Status,
		HandleNote:       row.HandleNote,
		HandledBy:        row.HandledBy,
		HandledAt:        row.HandledAt,
	}
}

// toPOVideoPart 把分P领域实体转为 PO。
func toPOVideoPart(part domain.VideoPart, videoID uint) po.VideoPart {
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
func toPOVideoResource(draft *domain.ResourceDraft, videoID uint) *po.VideoResource {
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
func toPOVideoPartResource(draft *domain.ResourceDraft, partID uint) *po.VideoPartResource {
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
