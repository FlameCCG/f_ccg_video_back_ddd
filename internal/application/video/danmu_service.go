package video

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// DanmuService 弹幕用例服务
// （原 service/video_service/{danmu_service.go,danmu_like.go,danmu_delete.go,
// danmu_room_list.go,danmu_admin_service.go,danmu_creator_service.go}）。
type DanmuService struct {
	videos      domain.VideoRepository
	danmus      domain.DanmuRepository
	query       DanmuQuery
	recent      DanmuRecentCache
	stats       *StatsService
	authors     *AuthorService
	creators    CreatorAnalytics
	broadcaster DanmuBroadcaster
}

// NewDanmuService 构造弹幕用例服务。
//
// broadcaster 在 bootstrap 构造 WebSocket Hub 后经 SetBroadcaster 回填。
func NewDanmuService(
	videos domain.VideoRepository,
	danmus domain.DanmuRepository,
	query DanmuQuery,
	recent DanmuRecentCache,
	stats *StatsService,
	authors *AuthorService,
	creators CreatorAnalytics,
) *DanmuService {
	return &DanmuService{
		videos:   videos,
		danmus:   danmus,
		query:    query,
		recent:   recent,
		stats:    stats,
		authors:  authors,
		creators: creators,
	}
}

// SetBroadcaster 回填弹幕广播端口（打断 Hub 与 Service 的构造环）。
func (s *DanmuService) SetBroadcaster(broadcaster DanmuBroadcaster) {
	s.broadcaster = broadcaster
}

// EnsureVideoAvailable 对外暴露的发布态校验，供 WS/HTTP 入口复用。
func (s *DanmuService) EnsureVideoAvailable(ctx context.Context, videoID uint) error {
	return s.videos.EnsurePublished(orTODO(ctx), videoID)
}

// ResolveDanmuPart 校验视频与分P关系，返回可用的 VideoPartID（单视频返回 0）。
func (s *DanmuService) ResolveDanmuPart(ctx context.Context, videoID, partID uint) (uint, error) {
	if videoID == 0 {
		return 0, domain.ErrVideoIDRequired
	}

	row, err := s.danmus.PartAvailability(orTODO(ctx), videoID, partID)
	if err != nil {
		return 0, err
	}

	return domain.ResolveDanmuPart(partID, row)
}

// Send 发送弹幕。
func (s *DanmuService) Send(ctx context.Context, userID uint, params SendDanmuParams) (*DanmuDTO, error) {
	cmd, err := domain.NormalizeSendDanmu(userID, domain.SendDanmuCommand{
		VideoID:     params.VideoID,
		VideoPartID: params.VideoPartID,
		Content:     params.Content,
		TimeOffset:  params.TimeOffset,
		Color:       params.Color,
		FontSize:    params.FontSize,
		Position:    params.Position,
	})
	if err != nil {
		return nil, err
	}

	partID, err := s.ResolveDanmuPart(ctx, cmd.VideoID, cmd.VideoPartID)
	if err != nil {
		return nil, err
	}

	record := &domain.Danmu{
		UserID:      userID,
		VideoID:     cmd.VideoID,
		VideoPartID: partID,
		Content:     cmd.Content,
		TimeOffset:  cmd.TimeOffset,
		Color:       cmd.Color,
		FontSize:    cmd.FontSize,
		Position:    cmd.Position,
	}

	if err := s.danmus.Create(ctx, record); err != nil {
		return nil, err
	}

	if _, err := s.stats.ApplyDelta(ctx, cmd.VideoID, domain.StatsDelta{DanmuCount: 1}); err != nil {
		// 弹幕已落库，stats 失败不回滚发送结果，仅告警。
		zap.L().Warn("apply video stats delta after send danmu failed",
			zap.Error(err),
			zap.Uint("videoId", cmd.VideoID),
		)
	} else {
		s.stats.RecordHotScoreDelta(ctx, cmd.VideoID, domain.HotScoreDanmuWeight, 1)
	}

	if authorID, authorErr := s.authors.AuthorID(ctx, cmd.VideoID); authorErr == nil {
		_ = s.creators.IncrDanmu(ctx, authorID, 1)
	}

	dto := toDanmuDTO(record)
	s.recent.Push(ctx, &dto)

	if s.broadcaster != nil {
		s.broadcaster.BroadcastDanmu(ctx, &dto)
	}

	return &dto, nil
}

// History 历史弹幕查询。
// 时间区间/排序以 DB 为准，不用 recent list 作为 History 权威数据源。
func (s *DanmuService) History(ctx context.Context, params DanmuHistoryParams) ([]DanmuDTO, error) {
	if params.VideoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	partID, err := s.ResolveDanmuPart(ctx, params.VideoID, params.VideoPartID)
	if err != nil {
		return nil, err
	}

	start, end, limit := domain.NormalizeDanmuHistoryRange(params.Start, params.End, params.Limit)

	result, err := s.query.History(ctx, params.VideoID, partID, start, end, limit)
	if err != nil {
		return nil, err
	}

	if params.UserID != 0 && len(result) > 0 {
		ids := make([]uint, 0, len(result))
		for _, item := range result {
			if item.ID != 0 {
				ids = append(ids, item.ID)
			}
		}

		if len(ids) > 0 {
			likedSet, likeErr := s.danmus.LikedSet(ctx, params.UserID, ids)
			if likeErr != nil {
				zap.L().Warn("load user danmu likes failed",
					zap.Error(likeErr),
					zap.Uint("userId", params.UserID),
					zap.Uint("videoId", params.VideoID),
				)
			} else {
				for i := range result {
					if _, ok := likedSet[result[i].ID]; ok {
						result[i].IsLiked = true
					}
				}
			}
		}
	}

	return result, nil
}

// RoomList 房间弹幕列表；UseTimeRange 为 true 时按播放时间分段返回。
func (s *DanmuService) RoomList(
	ctx context.Context,
	params DanmuRoomListParams,
) ([]DanmuRoomItem, int64, error) {
	if params.VideoID == 0 {
		return nil, 0, domain.ErrVideoIDRequired
	}

	ctx = orTODO(ctx)

	partID, err := s.ResolveDanmuPart(ctx, params.VideoID, params.VideoPartID)
	if err != nil {
		return nil, 0, err
	}

	if params.Page <= 0 {
		params.Page = 1
	}

	if params.UseTimeRange {
		pageSize, segErr := domain.NormalizeDanmuRoomSegment(params.Start, params.End, params.PageSize)
		if segErr != nil {
			return nil, 0, segErr
		}

		params.PageSize = pageSize
	} else {
		// 历史弹幕：固定返回最近 500 条，忽略 page/pageSize
		params.PageSize = domain.DanmuRoomHistoryLimit
	}

	queryParams := DanmuRoomQueryParams{
		VideoID:      params.VideoID,
		VideoPartID:  partID,
		Page:         params.Page,
		PageSize:     params.PageSize,
		UseTimeRange: params.UseTimeRange,
		Start:        params.Start,
		End:          params.End,
	}

	if params.Date != "" {
		start, end, dateErr := domain.ParseDanmuDateRange(params.Date)
		if dateErr != nil {
			return nil, 0, dateErr
		}

		queryParams.HasDate = true
		queryParams.DateStart = start
		queryParams.DateEnd = end
	}

	items, total, err := s.query.RoomList(ctx, queryParams)
	if err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []DanmuRoomItem{}, 0, nil
	}

	if params.UserID != 0 && len(items) > 0 {
		ids := make([]uint, 0, len(items))
		for i := range items {
			ids = append(ids, items[i].ID)
		}

		likedSet, likeErr := s.danmus.LikedSet(ctx, params.UserID, ids)
		if likeErr != nil {
			zap.L().Warn("load user danmu likes failed", zap.Error(likeErr), zap.Uint("userId", params.UserID))
			// 点赞状态失败时仍返回弹幕列表，仅 isLiked 保持默认 false
		} else {
			for idx := range items {
				if _, ok := likedSet[items[idx].ID]; ok {
					items[idx].IsLiked = true
				}
			}
		}
	}

	return items, total, nil
}

// ToggleLike 点赞/取消点赞弹幕。
func (s *DanmuService) ToggleLike(ctx context.Context, userID, danmuID uint) (*DanmuLikeResult, error) {
	if userID == 0 {
		return nil, domain.ErrLoginRequired
	}

	if danmuID == 0 {
		return nil, domain.ErrDanmuIDRequired
	}

	ctx = orTODO(ctx)

	danmu, err := s.danmus.FindByID(ctx, danmuID)
	if err != nil {
		return nil, err
	}

	liked, err := s.danmus.ToggleLike(ctx, danmuID, userID)
	if err != nil {
		return nil, err
	}

	// likeCount 变更后失效 recent 缓存，避免旁路读到旧计数。
	s.recent.Invalidate(ctx, danmu.VideoID, danmu.VideoPartID)

	likeCount, err := s.danmus.LikeCount(ctx, danmuID)
	if err != nil {
		return nil, err
	}

	return &DanmuLikeResult{LikeCount: likeCount, IsLiked: liked}, nil
}

// DeleteByUser 用户删除自己的弹幕。
func (s *DanmuService) DeleteByUser(ctx context.Context, userID, danmuID uint) error {
	if userID == 0 {
		return domain.ErrLoginRequired
	}

	if danmuID == 0 {
		return domain.ErrDanmuIDRequired
	}

	ctx = orTODO(ctx)

	danmu, err := s.danmus.FindByID(ctx, danmuID)
	if err != nil {
		return err
	}

	if !danmu.OwnedBy(userID) {
		return domain.ErrDanmuDeleteForbidden
	}

	return s.deleteDanmus(ctx, []domain.Danmu{*danmu})
}

// AdminDelete 管理员批量删除弹幕。
func (s *DanmuService) AdminDelete(ctx context.Context, danmuIDs []uint) (int64, error) {
	if len(danmuIDs) == 0 {
		return 0, domain.ErrDanmuIDsRequired
	}

	ctx = orTODO(ctx)

	danmus, err := s.danmus.FindByIDs(ctx, danmuIDs)
	if err != nil {
		return 0, err
	}

	if len(danmus) == 0 {
		return 0, nil
	}

	if err := s.deleteDanmus(ctx, danmus); err != nil {
		return 0, err
	}

	return int64(len(danmus)), nil
}

// AdminList 管理员弹幕列表。
func (s *DanmuService) AdminList(
	ctx context.Context,
	params AdminDanmuListParams,
) ([]AdminDanmuItem, int64, error) {
	if params.Page <= 0 {
		params.Page = 1
	}

	if params.PageSize <= 0 || params.PageSize > 100 {
		params.PageSize = 20
	}

	params.Keyword = domain.TrimKeyword(params.Keyword)
	params.Username = domain.TrimKeyword(params.Username)

	return s.query.AdminList(ctx, params)
}

// CreatorList 创作中心查看本人视频的弹幕。
func (s *DanmuService) CreatorList(
	ctx context.Context,
	authorID uint,
	page, pageSize, sort int,
	keyword string,
) ([]CreatorDanmuItem, int64, error) {
	if authorID == 0 {
		return nil, 0, domain.ErrParamInvalid
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}

	username := s.authors.UsernameByID(ctx, authorID)
	if username == "" {
		return nil, 0, domain.ErrUserNotFound
	}

	return s.query.CreatorList(ctx, username, page, pageSize, sort, domain.TrimKeyword(keyword))
}

// deleteDanmus 删除弹幕并同步视频/分P 弹幕数、热度、创作中心指标与缓存。
func (s *DanmuService) deleteDanmus(ctx context.Context, danmus []domain.Danmu) error {
	if len(danmus) == 0 {
		return nil
	}

	ctx = orTODO(ctx)

	ids := make([]uint, 0, len(danmus))
	videoDelta := make(map[uint]int64)
	partDelta := make(map[uint]int64)

	for _, d := range danmus {
		if d.ID == 0 {
			continue
		}

		ids = append(ids, d.ID)

		if d.VideoID != 0 {
			videoDelta[d.VideoID]++
		}

		if d.VideoPartID != 0 {
			partDelta[d.VideoPartID]++
		}
	}

	if len(ids) == 0 {
		return nil
	}

	if err := s.danmus.DeleteMany(ctx, ids); err != nil {
		return err
	}

	// 同步 Redis 视频统计与弹幕缓存（最佳努力）
	for videoID, dec := range videoDelta {
		if dec <= 0 {
			continue
		}

		if _, err := s.stats.ApplyDelta(ctx, videoID, domain.StatsDelta{DanmuCount: -dec}); err != nil {
			zap.L().Warn("apply video stats delta after delete danmu failed",
				zap.Error(err),
				zap.Uint("videoId", videoID),
				zap.Int64("danmuDelta", -dec),
			)
		} else {
			s.stats.RecordHotScoreDelta(ctx, videoID, domain.HotScoreDanmuWeight, -dec)
		}

		if authorID, err := s.authors.AuthorID(ctx, videoID); err == nil {
			_ = s.creators.IncrDanmu(ctx, authorID, -dec)
		}
	}

	for partID, dec := range partDelta {
		if dec <= 0 {
			continue
		}

		_ = s.danmus.DecrPartDanmuCount(ctx, partID, dec)
	}

	for _, d := range danmus {
		if d.VideoID == 0 {
			continue
		}

		s.recent.Invalidate(ctx, d.VideoID, d.VideoPartID)
	}

	return nil
}

// toDanmuDTO 把弹幕实体转为响应 DTO。
func toDanmuDTO(record *domain.Danmu) DanmuDTO {
	return DanmuDTO{
		ID:          record.ID,
		VideoID:     record.VideoID,
		VideoPartID: record.VideoPartID,
		UserID:      record.UserID,
		Content:     record.Content,
		TimeOffset:  record.TimeOffset,
		Color:       record.Color,
		FontSize:    record.FontSize,
		Position:    record.Position,
		LikeCount:   record.LikeCount,
		IsLiked:     false,
		CreatedAt:   record.CreatedAt,
	}
}
