package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// DetailQuery 视频详情读模型查询的 GORM 实现。
//
// 富嵌套读模型（作者/分区/标签/资源/分P）直接从 PO 组装为 application 层 DTO，
// json tag 与原响应逐字一致（见 architecture.md 第 4 节）。
type DetailQuery struct {
	db *gorm.DB
}

var _ app.DetailQuery = (*DetailQuery)(nil)

// NewDetailQuery 构造视频详情查询。
func NewDetailQuery(db *gorm.DB) *DetailQuery {
	return &DetailQuery{db: db}
}

// preloadAssociations 预载详情所需的全部关联（顺序与原 preloadVideoAssociations 一致）。
func preloadAssociations(db *gorm.DB) *gorm.DB {
	return db.
		Preload("Tags").
		Preload("VideoPartition").
		Preload("VideoResources").
		Preload("VideoParts", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("sort_order ASC, id ASC")
		}).
		Preload("VideoParts.Resources").
		Preload("Author")
}

// LoadPublished 读取已发布视频的详情 meta 与统计；不存在时返回 domain.ErrRecordNotFound。
func (q *DetailQuery) LoadPublished(
	ctx context.Context,
	videoID uint,
) (*app.DetailMeta, *domain.Stats, error) {
	var row po.Video
	if err := preloadAssociations(gormx.DB(ctx, q.db)).
		First(&row, "id = ? AND status = ?", videoID, enum.VideoStatusPublished).Error; err != nil {
		return nil, nil, mapNotFound(err)
	}

	meta, stats := buildMetaAndStats(&row)

	return meta, stats, nil
}

// LoadOwnerPreview 读取作者本人的非公开（私密/待审/定时）视频详情；
// 不存在时返回 domain.ErrRecordNotFound。
func (q *DetailQuery) LoadOwnerPreview(
	ctx context.Context,
	videoID, viewerID uint,
) (*app.DetailMeta, *domain.Stats, error) {
	var row po.Video
	if err := preloadAssociations(gormx.DB(ctx, q.db)).
		Select("videos.*").
		Joins("JOIN users owner ON owner.username = videos.author_username").
		Where(
			"videos.id = ? AND videos.status IN ? AND owner.id = ?",
			videoID,
			domain.OwnerPreviewStatuses(),
			viewerID,
		).
		First(&row).Error; err != nil {
		return nil, nil, mapNotFound(err)
	}

	meta, stats := buildMetaAndStats(&row)

	return meta, stats, nil
}

// LoadAny 读取任意状态的视频详情（管理端使用）；不存在时返回 domain.ErrRecordNotFound。
func (q *DetailQuery) LoadAny(
	ctx context.Context,
	videoID uint,
) (*app.DetailMeta, *domain.Stats, error) {
	var row po.Video
	if err := preloadAssociations(gormx.DB(ctx, q.db)).
		First(&row, "id = ?", videoID).Error; err != nil {
		return nil, nil, mapNotFound(err)
	}

	meta, stats := buildMetaAndStats(&row)

	return meta, stats, nil
}

// UserFlags 读取当前用户对视频的点赞/收藏/投币标记。
//
// 与原 loadVideoUserFlags 一致：单条 Raw SQL 三个 EXISTS，失败返回零值。
func (q *DetailQuery) UserFlags(ctx context.Context, videoID, userID uint) domain.UserFlags {
	if videoID == 0 || userID == 0 {
		return domain.UserFlags{}
	}

	type dbRow struct {
		IsLiked     int8 `gorm:"column:is_liked"`
		IsFavorited int8 `gorm:"column:is_favorited"`
		IsCoined    int8 `gorm:"column:is_coined"`
	}

	var row dbRow
	if err := gormx.DB(ctx, q.db).Raw(`
SELECT
  EXISTS(SELECT 1 FROM video_likes WHERE video_id = ? AND user_id = ?) AS is_liked,
  EXISTS(
    SELECT 1
    FROM favorite_folder_videos ffv
    JOIN favorite_folders ff ON ff.id = ffv.favorite_folder_id
    WHERE ff.user_id = ? AND ffv.video_id = ?
  ) AS is_favorited,
  EXISTS(SELECT 1 FROM video_coins WHERE video_id = ? AND user_id = ?) AS is_coined
`, videoID, userID, userID, videoID, videoID, userID).Scan(&row).Error; err != nil {
		return domain.UserFlags{}
	}

	return domain.UserFlags{
		IsLiked:     row.IsLiked > 0,
		IsFavorited: row.IsFavorited > 0,
		IsCoined:    row.IsCoined > 0,
	}
}

// buildMetaAndStats 从视频 PO 组装详情 meta 与统计（逐字段对齐原 buildMetaAndStats）。
func buildMetaAndStats(row *po.Video) (*app.DetailMeta, *domain.Stats) {
	if row == nil {
		return &app.DetailMeta{}, &domain.Stats{}
	}

	meta := &app.DetailMeta{
		IsScheduled: row.IsScheduled,
		PublishAt:   row.PublishAt,
		ApprovedAt:  row.ApprovedAt,
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		Cover:       row.Cover,
		Duration:    row.Duration,
		IsOriginal:  row.IsOriginal,
		StorageType: row.StorageType,
		Status:      row.Status,
		CreatedAt:   row.CreatedAt,
		Author: app.AuthorBrief{
			ID:          row.Author.ID,
			Username:    fallbackUsername(row),
			Avatar:      row.Author.Avatar,
			Level:       row.Author.Level,
			Description: row.Author.Description,
		},
	}

	// MinIO 历史对象可能是 octet-stream + nosniff，Chrome 会拒绝播放；
	// 用 response-content-type 覆盖响应头。
	needContentTypeOverride := row.StorageType == enum.StorageTypeMinio

	if row.VideoPartition.ID != 0 {
		meta.Partition = app.PartitionBrief{
			ID:            row.VideoPartition.ID,
			Name:          row.VideoPartition.Name,
			Icon:          row.VideoPartition.Icon,
			IsActive:      row.VideoPartition.IsActive,
			IsSubmittable: row.VideoPartition.IsSubmittable,
		}
	}

	if len(row.Tags) > 0 {
		meta.Tags = make([]app.TagBrief, 0, len(row.Tags))
		for _, tag := range row.Tags {
			meta.Tags = append(meta.Tags, app.TagBrief{ID: tag.ID, Name: tag.Name})
		}
	}

	if len(row.VideoResources) > 0 {
		meta.Resources = make([]domain.ResourceBrief, 0, len(row.VideoResources))
		for _, res := range row.VideoResources {
			meta.Resources = append(meta.Resources, toResourceBrief(
				res.ID, res.Resolution, res.SourceFileName, res.FileURL,
				res.FileSize, res.Bitrate, res.Format, res.Codec,
				res.IsVip, res.IsSouceVideo, needContentTypeOverride,
			))
		}

		meta.Resources = domain.DedupeResourceBriefs(meta.Resources)
		domain.SortResourceBriefs(meta.Resources)
	}

	if len(row.VideoParts) > 0 {
		meta.Parts = make([]app.PartBrief, 0, len(row.VideoParts))

		for _, part := range row.VideoParts {
			partBrief := app.PartBrief{
				ID:         part.ID,
				Title:      part.Title,
				SortOrder:  part.SortOrder,
				Duration:   part.Duration,
				DanmuCount: part.DanmuCount,
			}

			if len(part.Resources) > 0 {
				partBrief.Resources = make([]domain.ResourceBrief, 0, len(part.Resources))
				for _, res := range part.Resources {
					partBrief.Resources = append(partBrief.Resources, toResourceBrief(
						res.ID, res.Resolution, res.SourceFileName, res.FileURL,
						res.FileSize, res.Bitrate, res.Format, res.Codec,
						res.IsVip, res.IsSouceVideo, needContentTypeOverride,
					))
				}

				partBrief.Resources = domain.DedupeResourceBriefs(partBrief.Resources)
				domain.SortResourceBriefs(partBrief.Resources)
			}

			meta.Parts = append(meta.Parts, partBrief)
		}

		// 兼容旧前端：若是分P作品，默认把 P1 的资源挂到 resources 字段
		if len(meta.Resources) == 0 && len(meta.Parts) > 0 {
			meta.Resources = meta.Parts[0].Resources
		}
	}

	NormalizeSinglePartDetailMeta(meta)

	return meta, &domain.Stats{
		Views:         row.Views,
		Likes:         row.Likes,
		CommentCount:  row.CommentCount,
		CoinCount:     row.CoinCount,
		FavoriteCount: row.FavoriteCount,
		DanmuCount:    row.DanmuCount,
	}
}

// toResourceBrief 组装单条资源摘要（needContentTypeOverride 为 true 时修正播放 URL）。
func toResourceBrief(
	id uint,
	resolution enum.VideoResolution,
	sourceFileName, fileURL string,
	fileSize int64,
	bitrate int,
	format, codec string,
	isVip, isSource, needContentTypeOverride bool,
) domain.ResourceBrief {
	url := fileURL
	if needContentTypeOverride {
		url = domain.EnsurePlayableContentTypeURL(url)
	}

	return domain.ResourceBrief{
		ID:             id,
		Resolution:     resolution,
		SourceFileName: sourceFileName,
		FileURL:        url,
		FileSize:       fileSize,
		Bitrate:        bitrate,
		Format:         format,
		Codec:          codec,
		IsVip:          isVip,
		IsSource:       isSource,
	}
}

// NormalizeSinglePartDetailMeta 单分P作品把资源提升到 resources 并清空 parts。
//
// 该归一化在「读库组装」与「读 Redis meta 缓存」两条路径都要执行，
// 因此导出供 cache/video 复用（与原 normalizeSinglePartDetailMeta 逐行一致）。
func NormalizeSinglePartDetailMeta(meta *app.DetailMeta) {
	if meta == nil || len(meta.Parts) != 1 {
		return
	}

	if len(meta.Resources) == 0 {
		meta.Resources = meta.Parts[0].Resources
	}

	meta.Parts = nil
}

// fallbackUsername 作者关联缺失时回落 videos.author_username。
func fallbackUsername(row *po.Video) string {
	if row == nil {
		return ""
	}

	if row.Author.Username != "" {
		return row.Author.Username
	}

	return row.AuthorUsername
}
