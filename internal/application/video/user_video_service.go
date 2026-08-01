package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// UserVideoService 用户主页视频与最近互动用例服务
// （原 service/video_service/user_video_service.go）。
type UserVideoService struct {
	videos  UserVideoQuery
	authors *AuthorService
	history domain.HistoryRepository
	stats   *StatsService
}

// NewUserVideoService 构造用户主页视频用例服务。
func NewUserVideoService(
	videos UserVideoQuery,
	authors *AuthorService,
	history domain.HistoryRepository,
	stats *StatsService,
) *UserVideoService {
	return &UserVideoService{videos: videos, authors: authors, history: history, stats: stats}
}

// ListUserVideos 用户主页投稿分页列表。
//
// 本人查看时额外返回审核态字段；已发布视频的播放量/弹幕量以 Redis 权威计数覆盖库值。
func (s *UserVideoService) ListUserVideos(
	ctx context.Context,
	userID, viewerID uint,
	page, pageSize, sort, auditStatus int,
) ([]UserVideoItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserIDRequired
	}

	page, pageSize = domain.NormalizeUserVideoPaging(page, pageSize)

	username := s.authors.UsernameByID(ctx, userID)
	if username == "" {
		return nil, 0, domain.ErrUserNotFound
	}

	isOwnerView := viewerID > 0 && viewerID == userID

	rows, total, err := s.videos.ListByAuthor(ctx, username, isOwnerView, auditStatus, sort, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []UserVideoItem{}, 0, nil
	}

	progress := s.progressMap(ctx, viewerID, rows)
	statsMap := s.publishedStats(ctx, rows)

	items := make([]UserVideoItem, 0, len(rows))

	for _, row := range rows {
		views, danmu := row.Views, row.DanmuCount
		if stats, ok := statsMap[row.ID]; ok && stats != nil {
			views = stats.Views
			danmu = stats.DanmuCount
		}

		item := UserVideoItem{
			ID:         row.ID,
			Title:      row.Title,
			Cover:      row.Cover,
			Views:      views,
			DanmuCount: danmu,
			Duration:   row.Duration,
			Progress:   progress[row.ID],
			CreatedAt:  row.CreatedAt,
			Status:     int(row.Status),
			StatusText: domain.MapUserVideoStatusText(row.Status),
		}
		if isOwnerView {
			item.AuditStatus, item.AuditStatusText = domain.MapUserVideoAuditStatus(row.Status)
		}

		items = append(items, item)
	}

	return items, total, nil
}

// RecentActions 返回用户最近点赞/投币的视频，限制 limit 条。
func (s *UserVideoService) RecentActions(
	ctx context.Context,
	userID uint,
	action string,
	limit int,
) ([]UserVideoItem, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	limit = domain.NormalizeRecentActionLimit(limit)

	if err := domain.ValidateRecentActionType(action); err != nil {
		return nil, err
	}

	rows, err := s.videos.RecentActionVideos(ctx, userID, action, limit)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []UserVideoItem{}, nil
	}

	videoIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		if row.ID > 0 {
			videoIDs = append(videoIDs, row.ID)
		}
	}

	statsMap := s.stats.LoadBatch(ctx, videoIDs)

	items := make([]UserVideoItem, 0, len(rows))

	for _, row := range rows {
		views, danmu := row.Views, row.DanmuCount
		if stats, ok := statsMap[row.ID]; ok && stats != nil {
			views = stats.Views
			danmu = stats.DanmuCount
		}

		authorName := row.AuthorUsername
		if row.AuthorName != "" {
			authorName = row.AuthorName
		}

		items = append(items, UserVideoItem{
			ID:         row.ID,
			Title:      row.Title,
			Cover:      row.Cover,
			Views:      views,
			DanmuCount: danmu,
			Duration:   row.Duration,
			CreatedAt:  row.CreatedAt,
			AuthorID:   row.AuthorID,
			AuthorName: authorName,
		})
	}

	return items, nil
}

// progressMap 读取观看者在这批视频上的播放进度（未登录或无记录时为空）。
func (s *UserVideoService) progressMap(ctx context.Context, viewerID uint, rows []UserVideoRow) map[uint]int {
	if viewerID == 0 || len(rows) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	return s.history.ProgressByVideoIDs(ctx, viewerID, ids)
}

// publishedStats 仅对已发布视频取 Redis 权威计数（与原实现一致）。
func (s *UserVideoService) publishedStats(ctx context.Context, rows []UserVideoRow) map[uint]*domain.Stats {
	videoIDs := make([]uint, 0, len(rows))

	for _, row := range rows {
		if row.Status == enum.VideoStatusPublished && row.ID > 0 {
			videoIDs = append(videoIDs, row.ID)
		}
	}

	if len(videoIDs) == 0 {
		return nil
	}

	return s.stats.LoadBatch(ctx, videoIDs)
}
