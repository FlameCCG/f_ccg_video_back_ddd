package identity

import (
	"errors"
	"testing"
	"time"
)

func TestCreatorAnalyticsRangeAliasesAndMetricContract(t *testing.T) {
	policy := CreatorAnalyticsPolicy{}
	zone := time.FixedZone("test", 8*60*60)
	now := time.Date(2024, 2, 29, 14, 30, 0, 0, zone)
	for _, tc := range []struct {
		raw, normalized string
		count           int
	}{{"", "7d", 7}, {"week", "7d", 7}, {"7", "7d", 7}, {"7d", "7d", 7}, {"30", "30d", 30}, {"30d", "30d", 30}, {"month", "month", 29}, {"1m", "month", 29}} {
		days, normalized, err := policy.RangeDays(tc.raw, now)
		if err != nil || normalized != tc.normalized || len(days) != tc.count {
			t.Fatalf("range %q: %s, %v, %v", tc.raw, normalized, days, err)
		}
		for i, day := range days {
			want := time.Date(2024, 2, 29, 0, 0, 0, 0, zone).AddDate(0, 0, i-tc.count+1)
			if !day.Equal(want) || day.Location() != zone {
				t.Fatalf("range %q date %d: %v, want %v", tc.raw, i, day, want)
			}
		}
	}
	if _, _, err := policy.RangeDays("year", now); !errors.Is(err, ErrCreatorRangeInvalid) {
		t.Fatalf("invalid range: %v", err)
	}
	if got, err := policy.NormalizeMetric(""); got != CreatorMetricViews || err != nil {
		t.Fatalf("default metric: %q, %v", got, err)
	}
	for _, metric := range []string{CreatorMetricViews, CreatorMetricFans, CreatorMetricComments, CreatorMetricCoins, CreatorMetricDanmu, CreatorMetricFavorites} {
		if got, err := policy.NormalizeMetric(metric); got != metric || err != nil {
			t.Fatalf("metric %q: %q, %v", metric, got, err)
		}
	}
	if _, err := policy.NormalizeMetric("Views"); !errors.Is(err, ErrCreatorMetricTypeInvalid) {
		t.Fatalf("metric case sensitivity changed: %v", err)
	}
	if got := policy.Total([]int64{5, -2, 9}); got != 12 {
		t.Fatal(got)
	}
}
