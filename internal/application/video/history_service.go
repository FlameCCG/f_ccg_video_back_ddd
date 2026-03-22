package video

import (
	"context"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// HistoryService 播放历史用例服务。
type HistoryService struct {
	videos  VideoFactsQuery
	history videodomain.HistoryRepository
	query   HistoryQuery
}

// NewHistoryService 构造播放历史用例服务。
func NewHistoryService(
	videos VideoFactsQuery,
	history videodomain.HistoryRepository,
	query HistoryQuery,
) *HistoryService {
	return &HistoryService{videos: videos, history: history, query: query}
}

// SavePlayHistory 保存/更新播放进度。
func (s *HistoryService) SavePlayHistory(ctx context.Context, userID, videoID uint, progress, duration int) error {
	if userID == 0 {
		return videodomain.ErrUserNotLogin
	}

	if videoID == 0 {
		return videodomain.ErrVideoIDRequired
	}

	exists, err := s.videos.Exists(ctx, videoID)
	if err != nil {
		return err
	}

	if !exists {
		return videodomain.ErrVideoNotFound
	}

	return s.history.Upsert(ctx, userID, videoID, progress, duration)
}

// GetPlayHistory 查询播放进度。
func (s *HistoryService) GetPlayHistory(ctx context.Context, userID, videoID uint) (*videodomain.PlayHistory, error) {
	if userID == 0 || videoID == 0 {
		return nil, videodomain.ErrParamInvalid
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
		return nil, 0, videodomain.ErrUserNotLogin
	}

	page, pageSize = (videodomain.ListingPolicy{}).UserVideoPage(page, pageSize)

	return s.query.List(ctx, userID, page, pageSize, (videodomain.ListingPolicy{}).Keyword(keyword))
}

// DeleteHistory 批量删除用户历史记录。
func (s *HistoryService) DeleteHistory(ctx context.Context, userID uint, videoIDs []uint) error {
	if userID == 0 {
		return videodomain.ErrUserNotLogin
	}

	if len(videoIDs) == 0 {
		return nil
	}

	return s.history.DeleteByVideoIDs(ctx, userID, videoIDs)
}
