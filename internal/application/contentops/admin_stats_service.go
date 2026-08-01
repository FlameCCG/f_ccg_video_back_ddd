package contentops

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
)

// 管理端统计窗口（与原 service/site_service/admin_stats_service.go 一致）。
const (
	// adminStatsDailyDays 日维度窗口：最近一周（含今天）。
	adminStatsDailyDays = 7
	// adminStatsMonthlyMonths 月维度窗口：最近一年（含本月）。
	adminStatsMonthlyMonths = 12
)

// AdminStats 管理端站点统计（总览 + 日维度趋势 + 月维度趋势）。
//
// 错误处理语义与原实现完全一致：
// 四项总量统计失败直接返回错误；在线人数、以及日/月循环内的逐项统计
// 失败一律忽略并记 0（含当日登录去重数），保证接口整体可用。
func (s *StatService) AdminStats(ctx context.Context) (*AdminSiteStats, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	totalUsers, err := s.query.CountUsers(ctx)
	if err != nil {
		return nil, err
	}

	totalVideos, err := s.query.CountPublishedVideos(ctx)
	if err != nil {
		return nil, err
	}

	totalComments, err := s.query.CountComments(ctx)
	if err != nil {
		return nil, err
	}

	todayRegisters, err := s.query.CountUsersCreatedBetween(ctx, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}

	var onlineUsers int64
	if v, onlineErr := s.store.OnlineCount(ctx); onlineErr == nil {
		onlineUsers = v
	}

	// daily (最近一周，含今天，共 7 天)
	dailyLabels := make([]string, 0, adminStatsDailyDays)
	visitValues := make([]int64, 0, adminStatsDailyDays)
	loginValues := make([]int64, 0, adminStatsDailyDays)
	videoValues := make([]int64, 0, adminStatsDailyDays)
	userValues := make([]int64, 0, adminStatsDailyDays)

	for i := adminStatsDailyDays - 1; i >= 0; i-- {
		dayStart := todayStart.AddDate(0, 0, -i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		dailyLabels = append(dailyLabels, dayStart.Format("2006-01-02"))

		uv := int64(0)
		if v, uvErr := s.store.DailyUV(ctx, dayStart); uvErr == nil {
			uv = v
		}

		visitValues = append(visitValues, uv)

		loginUsers, _ := s.query.CountDistinctLoginUsersBetween(ctx, dayStart, dayEnd)
		loginValues = append(loginValues, loginUsers)

		publishVideos, _ := s.query.CountPublishedVideosCreatedBetween(ctx, dayStart, dayEnd)
		videoValues = append(videoValues, publishVideos)

		newUsers, _ := s.query.CountUsersCreatedBetween(ctx, dayStart, dayEnd)
		userValues = append(userValues, newUsers)
	}

	// 今日登录数就是日序列的末位（循环最后一轮 i=0 即今天），
	// 单独再查一次只会多一条 SQL，还可能与趋势末位对不上。
	todayLoginUsers := loginValues[len(loginValues)-1]

	// monthly (最近一年，含本月，共 12 月)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	yearStart := monthStart.AddDate(0, -(adminStatsMonthlyMonths - 1), 0)
	monthlyLabels := make([]string, 0, adminStatsMonthlyMonths)
	monthlyVideoValues := make([]int64, 0, adminStatsMonthlyMonths)

	for i := range adminStatsMonthlyMonths {
		start := yearStart.AddDate(0, i, 0)
		end := start.AddDate(0, 1, 0)
		monthlyLabels = append(monthlyLabels, start.Format("2006-01"))

		newVideos, _ := s.query.CountPublishedVideosCreatedBetween(ctx, start, end)
		monthlyVideoValues = append(monthlyVideoValues, newVideos)
	}

	stats := &AdminSiteStats{
		Overview: AdminOverview{
			TotalUsers:      totalUsers,
			OnlineUsers:     onlineUsers,
			TotalVideos:     totalVideos,
			TotalComments:   totalComments,
			TodayLoginUsers: todayLoginUsers,
			TodayRegisters:  todayRegisters,
		},
		Daily: AdminDailyStats{
			VisitUsers: TrendSeries{
				X:      dailyLabels,
				Values: visitValues,
				Rates:  domain.CalcRates(visitValues),
			},
			LoginUsers: TrendSeries{
				X:      dailyLabels,
				Values: loginValues,
				Rates:  domain.CalcRates(loginValues),
			},
			PublishVideos: TrendSeries{
				X:      dailyLabels,
				Values: videoValues,
				Rates:  domain.CalcRates(videoValues),
			},
			NewUsers: TrendSeries{
				X:      dailyLabels,
				Values: userValues,
				Rates:  domain.CalcRates(userValues),
			},
		},
		Monthly: AdminMonthlyStats{
			NewVideos: TrendSeries{
				X:      monthlyLabels,
				Values: monthlyVideoValues,
				Rates:  domain.CalcRates(monthlyVideoValues),
			},
		},
	}

	return stats, nil
}
