package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// DetailService 视频详情用例服务（原 service/video_service/detail_service.go）。
//
// 读侧走 CQRS：富嵌套读模型由 persistence 从 PO 组装，
// meta 与 stats 分别落两套 Redis 缓存（meta 有 TTL + generation CAS，stats 为权威计数）。
type DetailService struct {
	query VideoDetailQuery
	meta  DetailMetaCache
	stats *StatsService
	dash  DASHVariantResolver
}

// NewDetailService 构造视频详情用例服务。
func NewDetailService(
	query VideoDetailQuery,
	meta DetailMetaCache,
	stats *StatsService,
	dash DASHVariantResolver,
) *DetailService {
	return &DetailService{query: query, meta: meta, stats: stats, dash: dash}
}

// GetVideoDetail 获取视频详情（已发布优先；作者本人可预览私密/待审/定时稿）。
func (s *DetailService) GetVideoDetail(ctx context.Context, videoID, viewerID uint) (*VideoDetail, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	if meta, _ := s.meta.Load(ctx, videoID); meta != nil && meta.Status == videoStatusPublished {
		if stats, ok := s.stats.cache.Load(ctx, videoID); ok && stats != nil {
			return s.mergeDetail(ctx, meta, stats, viewerID), nil
		}

		dbStats, err := s.stats.videos.PublishedStats(ctx, videoID)
		if err != nil {
			return nil, err
		}

		s.stats.cache.Seed(ctx, videoID, dbStats)

		return s.mergeDetail(ctx, meta, dbStats, viewerID), nil
	}

	// generation 必须在 load DB 之前捕获。
	// 若 load 后再读 gen，Invalidate/bump 后会拿到新 gen，旧 Published/无 DASH 快照仍可 CAS 写回。
	gen := s.meta.Generation(ctx, videoID)

	meta, stats, err := s.loadForViewer(ctx, videoID, viewerID)
	if err != nil {
		return nil, err
	}

	if meta.Status == videoStatusPublished {
		s.meta.StoreIfGenerationMatch(ctx, videoID, meta, gen)
		s.stats.cache.Seed(ctx, videoID, stats)
	}

	return s.mergeDetail(ctx, meta, stats, viewerID), nil
}

// GetVideoDetailAdmin 管理员获取视频详情（不过滤视频状态，不使用缓存）。
//
// 管理端资源清单会解析 DASH MPD，展开各 Representation（分辨率/码率/文件名）。
func (s *DetailService) GetVideoDetailAdmin(ctx context.Context, videoID uint) (*VideoDetail, error) {
	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	meta, stats, err := s.query.LoadAny(ctx, videoID)
	if err != nil {
		return nil, err
	}

	if meta != nil {
		s.enrichDASHVariants(ctx, meta.Resources)

		for i := range meta.Parts {
			s.enrichDASHVariants(ctx, meta.Parts[i].Resources)
		}
	}

	return s.mergeDetail(ctx, meta, stats, 0), nil
}

func (s *DetailService) loadForViewer(
	ctx context.Context,
	videoID, viewerID uint,
) (*DetailMeta, *domain.Stats, error) {
	meta, stats, err := s.query.LoadPublished(ctx, videoID)
	if err == nil {
		return meta, stats, nil
	}

	if !isRecordNotFound(err) || viewerID == 0 {
		return nil, nil, err
	}

	return s.query.LoadOwnerPreview(ctx, videoID, viewerID)
}

func (s *DetailService) mergeDetail(
	ctx context.Context,
	meta *DetailMeta,
	stats *domain.Stats,
	userID uint,
) *VideoDetail {
	if meta == nil {
		return nil
	}

	detail := &VideoDetail{
		ID:          meta.ID,
		Title:       meta.Title,
		Description: meta.Description,
		Cover:       meta.Cover,
		Duration:    meta.Duration,
		IsOriginal:  meta.IsOriginal,
		StorageType: meta.StorageType,
		Status:      meta.Status,
		CreatedAt:   meta.CreatedAt,
		Partition:   meta.Partition,
		Author:      meta.Author,
		Tags:        meta.Tags,
		Resources:   meta.Resources,
		Parts:       meta.Parts,
	}

	if stats != nil {
		detail.Views = stats.Views
		detail.Likes = stats.Likes
		detail.CommentCount = stats.CommentCount
		detail.CoinCount = stats.CoinCount
		detail.FavoriteCount = stats.FavoriteCount
		detail.DanmuCount = stats.DanmuCount
	}

	if userID > 0 {
		flags := s.query.UserFlags(ctx, meta.ID, userID)
		detail.IsLiked = flags.IsLiked
		detail.IsFavorited = flags.IsFavorited
		detail.IsCoined = flags.IsCoined
	}

	return detail
}

// enrichDASHVariants 逐个解析 DASH 资源的 MPD（最佳努力，失败仅记录日志）。
func (s *DetailService) enrichDASHVariants(ctx context.Context, resources []domain.ResourceBrief) {
	if len(resources) == 0 || s.dash == nil {
		return
	}

	for i := range resources {
		if !domain.IsDASHResource(resources[i]) {
			continue
		}

		variants, err := s.dash.Variants(ctx, resources[i].ID, resources[i].FileURL)
		if err != nil {
			continue
		}

		resources[i].Variants = variants
		// 父行：库内 bitrate 为 0 时展示最大视频码率。
		if resources[i].Bitrate <= 0 {
			if maxBr := domain.MaxVariantBitrateKbps(variants, "video"); maxBr > 0 {
				resources[i].Bitrate = maxBr
			}
		}
	}
}
