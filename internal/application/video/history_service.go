package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// HistoryService 播放历史用例服务（原 service/video_service/history_service.go）。
type HistoryService struct {
	videos  domain.VideoRepository
	history domain.HistoryRepository
	query   HistoryQuery
}

// NewHistoryService 构造播放历史用例服务。
func NewHistoryService(
	videos domain.VideoRepository,
	history domain.HistoryRepository,
	query HistoryQuery,
) *HistoryService {
	return &HistoryService{videos: videos, history: history, query: query}
}

// SavePlayHistory 保存/更新播放进度。
func (s *HistoryService) SavePlayHistory(ctx context.Context, userID, videoID uint, progress, duration int) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if videoID == 0 {
		return domain.ErrVideoIDRequired
	}

	exists, err := s.videos.Exists(ctx, videoID)
	if err != nil {
		return err
	}

	if !exists {
		return domain.ErrVideoNotFound
	}

	return s.history.Upsert(ctx, userID, videoID, progress, duration)
}

// GetPlayHistory 查询播放进度。
func (s *HistoryService) GetPlayHistory(ctx context.Context, userID, videoID uint) (*domain.PlayHistory, error) {
	if userID == 0 || videoID == 0 {
		return nil, domain.ErrParamInvalid
	}

	return s.history.Find(ctx, userID, videoID)
}

// ListUserHistory 获取用户观看历史列表。
func (s *HistoryService) ListUserHistory(
	ctx context.Context,
	userID uint,
	page, pageSize int,
	keyword string,
) ([]HistoryItem, int64, error) {
	if userID == 0 {
		return nil, 0, domain.ErrUserNotLogin
	}

	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	return s.query.List(ctx, userID, page, pageSize, domain.TrimKeyword(keyword))
}

// DeleteHistory 批量删除用户历史记录。
func (s *HistoryService) DeleteHistory(ctx context.Context, userID uint, videoIDs []uint) error {
	if userID == 0 {
		return domain.ErrUserNotLogin
	}

	if len(videoIDs) == 0 {
		return nil
	}

	return s.history.DeleteByVideoIDs(ctx, userID, videoIDs)
}
