package identity

import identitydomain "f_ccg_video_back_ddd/internal/domain/identity"

// CreatorOverview 创作中心六项总览（读模型）.
type CreatorOverview struct {
	Fans      int64 `json:"fans"`
	Views     int64 `json:"views"`
	Comments  int64 `json:"comments"`
	Coins     int64 `json:"coins"`
	Danmu     int64 `json:"danmu"`
	Favorites int64 `json:"favorites"`
}

// CreatorAnalyticsTrendResult 创作中心趋势结果（读模型）.
type CreatorAnalyticsTrendResult struct {
	Range  string   `json:"range"`
	Type   string   `json:"type"`
	X      []string `json:"x"`
	Values []int64  `json:"values"`
	Total  int64    `json:"total"`
}

// CreatorVideoAggregate 作者已发布视频的统计汇总（读模型，含 Redis 统计缓存优先）.
type CreatorVideoAggregate struct {
	Views     int64
	Comments  int64
	Coins     int64
	Danmu     int64
	Favorites int64
}

// 创作中心指标类型。
const (
	// CreatorAnalyticsTypeFans 粉丝数指标.
	CreatorAnalyticsTypeFans = identitydomain.CreatorMetricFans
	// CreatorAnalyticsTypeViews 播放量指标.
	CreatorAnalyticsTypeViews = identitydomain.CreatorMetricViews
	// CreatorAnalyticsTypeComments 评论量指标.
	CreatorAnalyticsTypeComments = identitydomain.CreatorMetricComments
	// CreatorAnalyticsTypeCoins 投币量指标.
	CreatorAnalyticsTypeCoins = identitydomain.CreatorMetricCoins
	// CreatorAnalyticsTypeDanmu 弹幕量指标.
	CreatorAnalyticsTypeDanmu = identitydomain.CreatorMetricDanmu
	// CreatorAnalyticsTypeFavorites 收藏量指标.
	CreatorAnalyticsTypeFavorites = identitydomain.CreatorMetricFavorites
)
