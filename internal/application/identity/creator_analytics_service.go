package identity

import (
	"context"
	"errors"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
)

// CreatorAnalyticsService 创作中心数据分析用例服务
// （原 service/user_service/creator_analytics_service.go）。
type CreatorAnalyticsService struct {
	metrics CreatorMetricStore
	users   domain.UserRepository
	videos  CreatorVideoStatsQuery
	fans    FansCounter
}

// NewCreatorAnalyticsService 构造创作中心分析服务。
func NewCreatorAnalyticsService(
	metrics CreatorMetricStore,
	users domain.UserRepository,
	videos CreatorVideoStatsQuery,
	fans FansCounter,
) *CreatorAnalyticsService {
	return &CreatorAnalyticsService{metrics: metrics, users: users, videos: videos, fans: fans}
}

// ErrCreatorMetricTypeInvalid 指标类型非法（文案与原实现一致）。
var ErrCreatorMetricTypeInvalid = errors.New("type 仅支持 fans/views/comments/coins/danmu/favorites")

// ErrCreatorRangeInvalid 时间范围非法（文案与原实现一致）。
var ErrCreatorRangeInvalid = errors.New("range 仅支持 7d/30d/month")

// ErrCreatorIDRequired creatorId 缺失（文案与原实现一致）。
var ErrCreatorIDRequired = errors.New("creatorId 不能为空")

// IncrFans 粉丝数指标 +delta。
func (s *CreatorAnalyticsService) IncrFans(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeFans, creatorID, delta)
}

// IncrViews 播放量指标 +delta。
func (s *CreatorAnalyticsService) IncrViews(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeViews, creatorID, delta)
}

// IncrComments 评论量指标 +delta。
func (s *CreatorAnalyticsService) IncrComments(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeComments, creatorID, delta)
}

// IncrCoins 投币量指标 +delta。
func (s *CreatorAnalyticsService) IncrCoins(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeCoins, creatorID, delta)
}

// IncrDanmu 弹幕量指标 +delta。
func (s *CreatorAnalyticsService) IncrDanmu(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeDanmu, creatorID, delta)
}

// IncrFavorites 收藏量指标 +delta。
func (s *CreatorAnalyticsService) IncrFavorites(ctx context.Context, creatorID uint, delta int64) error {
	return s.metrics.Incr(ctx, CreatorAnalyticsTypeFavorites, creatorID, delta)
}

// Analytics 创作中心数据分析（按天聚合）。
// rangeKey: 7d / 30d / month（自然月）。
func (s *CreatorAnalyticsService) Analytics(
	ctx context.Context,
	creatorID uint,
	rangeKey, metricType string,
) (*CreatorAnalyticsTrendResult, error) {
	if err := s.metrics.Available(); err != nil {
		return nil, err
	}

	if creatorID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	days, normalized, err := buildRangeDays(rangeKey)
	if err != nil {
		return nil, err
	}

	metricType, err = normalizeCreatorMetricType(metricType)
	if err != nil {
		return nil, err
	}

	dayFields := make([]string, 0, len(days))

	labels := make([]string, 0, len(days))

	for _, d := range days {
		dayFields = append(dayFields, d.Format("20060102"))
		labels = append(labels, d.Format("2006-01-02"))
	}

	values, err := s.metrics.Series(ctx, metricType, creatorID, dayFields)
	if err != nil {
		return nil, err
	}

	return &CreatorAnalyticsTrendResult{
		Range:  normalized,
		Type:   metricType,
		X:      labels,
		Values: values,
		Total:  sumInt64(values),
	}, nil
}

// Overview 创作中心六项总览。
func (s *CreatorAnalyticsService) Overview(ctx context.Context, creatorID uint) (*CreatorOverview, error) {
	if creatorID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	username, err := s.users.UsernameByID(ctx, creatorID)
	if err != nil {
		return nil, err
	}

	if username == "" {
		return nil, domain.ErrUserNotFound
	}

	overview := &CreatorOverview{
		Fans: s.fans.FansCount(ctx, creatorID),
	}

	aggregate, err := s.videos.AggregateByAuthor(ctx, username)
	if err != nil {
		return nil, err
	}

	overview.Views = aggregate.Views
	overview.Comments = aggregate.Comments
	overview.Coins = aggregate.Coins
	overview.Danmu = aggregate.Danmu
	overview.Favorites = aggregate.Favorites

	return overview, nil
}

// buildRangeDays 按 rangeKey 展开日期序列，并返回规范化后的 range 名。
func buildRangeDays(rangeKey string) ([]time.Time, string, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	switch rangeKey {
	case "", "7d", "7", "week":
		start := todayStart.AddDate(0, 0, -6)

		return buildDaySeries(start, todayStart), "7d", nil
	case "30d", "30":
		start := todayStart.AddDate(0, 0, -29)

		return buildDaySeries(start, todayStart), "30d", nil
	case "month", "1m":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

		return buildDaySeries(start, todayStart), "month", nil
	default:
		return nil, "", ErrCreatorRangeInvalid
	}
}

func buildDaySeries(start, end time.Time) []time.Time {
	if end.Before(start) {
		start, end = end, start
	}

	days := int(end.Sub(start).Hours()/24) + 1

	out := make([]time.Time, 0, days)
	for i := range days {
		out = append(out, start.AddDate(0, 0, i))
	}

	return out
}

func normalizeCreatorMetricType(metricType string) (string, error) {
	switch metricType {
	case "", CreatorAnalyticsTypeViews:
		return CreatorAnalyticsTypeViews, nil
	case CreatorAnalyticsTypeFans:
		return CreatorAnalyticsTypeFans, nil
	case CreatorAnalyticsTypeComments:
		return CreatorAnalyticsTypeComments, nil
	case CreatorAnalyticsTypeCoins:
		return CreatorAnalyticsTypeCoins, nil
	case CreatorAnalyticsTypeDanmu:
		return CreatorAnalyticsTypeDanmu, nil
	case CreatorAnalyticsTypeFavorites:
		return CreatorAnalyticsTypeFavorites, nil
	default:
		return "", ErrCreatorMetricTypeInvalid
	}
}

func sumInt64(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}

	return total
}
