package identity

import (
	"context"
	"errors"
	"time"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
)

// CreatorAnalyticsService 创作中心数据分析用例服务。
type CreatorAnalyticsService struct {
	metrics CreatorMetricStore
	users   identitydomain.UserRepository
	videos  CreatorVideoStatsQuery
	fans    FansCounter
}

// NewCreatorAnalyticsService 构造创作中心分析服务。
func NewCreatorAnalyticsService(
	metrics CreatorMetricStore,
	users identitydomain.UserRepository,
	videos CreatorVideoStatsQuery,
	fans FansCounter,
) *CreatorAnalyticsService {
	return &CreatorAnalyticsService{metrics: metrics, users: users, videos: videos, fans: fans}
}

// ErrCreatorMetricTypeInvalid 指标类型非法。
var ErrCreatorMetricTypeInvalid = identitydomain.ErrCreatorMetricTypeInvalid

// ErrCreatorRangeInvalid 时间范围非法。
var ErrCreatorRangeInvalid = identitydomain.ErrCreatorRangeInvalid

// ErrCreatorIDRequired creatorId 缺失。
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
		return nil, identitydomain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	policy := identitydomain.CreatorAnalyticsPolicy{}
	days, normalized, err := policy.RangeDays(rangeKey, time.Now())
	if err != nil {
		return nil, err
	}

	metricType, err = policy.NormalizeMetric(metricType)
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
		Total:  policy.Total(values),
	}, nil
}

// Overview 创作中心六项总览。
func (s *CreatorAnalyticsService) Overview(ctx context.Context, creatorID uint) (*CreatorOverview, error) {
	if creatorID == 0 {
		return nil, identitydomain.ErrUserNotLogin
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	username, err := s.users.UsernameByID(ctx, creatorID)
	if err != nil {
		return nil, err
	}

	if username == "" {
		return nil, identitydomain.ErrUserNotFound
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
