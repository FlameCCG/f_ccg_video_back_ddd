package identity

import (
	"errors"
	"time"
)

var (
	ErrCreatorMetricTypeInvalid = errors.New("type 仅支持 fans/views/comments/coins/danmu/favorites")
	ErrCreatorRangeInvalid      = errors.New("range 仅支持 7d/30d/month")
)

const (
	CreatorMetricFans      = "fans"
	CreatorMetricViews     = "views"
	CreatorMetricComments  = "comments"
	CreatorMetricCoins     = "coins"
	CreatorMetricDanmu     = "danmu"
	CreatorMetricFavorites = "favorites"
)

// CreatorAnalyticsPolicy 维护创作中心指标口径与时间范围；时钟由用例提供。
type CreatorAnalyticsPolicy struct{}

func (p CreatorAnalyticsPolicy) RangeDays(rangeKey string, now time.Time) ([]time.Time, string, error) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch rangeKey {
	case "", "7d", "7", "week":
		return p.daySeries(today.AddDate(0, 0, -6), today), "7d", nil
	case "30d", "30":
		return p.daySeries(today.AddDate(0, 0, -29), today), "30d", nil
	case "month", "1m":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return p.daySeries(start, today), "month", nil
	default:
		return nil, "", ErrCreatorRangeInvalid
	}
}

func (CreatorAnalyticsPolicy) daySeries(start, end time.Time) []time.Time {
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

func (CreatorAnalyticsPolicy) NormalizeMetric(metric string) (string, error) {
	switch metric {
	case "", CreatorMetricViews:
		return CreatorMetricViews, nil
	case CreatorMetricFans, CreatorMetricComments, CreatorMetricCoins, CreatorMetricDanmu, CreatorMetricFavorites:
		return metric, nil
	default:
		return "", ErrCreatorMetricTypeInvalid
	}
}

func (CreatorAnalyticsPolicy) Total(values []int64) int64 {
	var total int64
	for _, value := range values {
		total += value
	}
	return total
}
