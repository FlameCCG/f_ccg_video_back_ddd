package contentops

import (
	"context"
	"time"
)

// AdminStats 管理端站点统计（总览 + 日维度趋势 + 月维度趋势）。
//
// 错误处理：四项总量统计失败直接返回错误；在线人数、以及日/月循环内的逐项统计
// 失败一律忽略并记 0（含当日登录去重数），保证接口整体可用。
func (s *StatService) AdminStats(ctx context.Context) (*AdminSiteStats, error) {
	windows := s.rates.Windows(time.Now())

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

	todayRegisters, err := s.query.CountUsersCreatedBetween(ctx, windows.Today.Start, windows.Today.End)
	if err != nil {
		return nil, err
	}

	var onlineUsers int64
	if v, onlineErr := s.store.OnlineCount(ctx); onlineErr == nil {
		onlineUsers = v
	}

	// daily (最近一周，含今天，共 7 天)
	dailyLabels := make([]string, 0, len(windows.Daily))
	visitValues := make([]int64, 0, len(windows.Daily))
	loginValues := make([]int64, 0, len(windows.Daily))
	videoValues := make([]int64, 0, len(windows.Daily))
	userValues := make([]int64, 0, len(windows.Daily))

	for _, period := range windows.Daily {
		dayStart, dayEnd := period.Start, period.End
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
	monthlyLabels := make([]string, 0, len(windows.Monthly))
	monthlyVideoValues := make([]int64, 0, len(windows.Monthly))

	for _, period := range windows.Monthly {
		start, end := period.Start, period.End
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
				Rates:  s.rates.CalcRates(visitValues),
			},
			LoginUsers: TrendSeries{
				X:      dailyLabels,
				Values: loginValues,
				Rates:  s.rates.CalcRates(loginValues),
			},
			PublishVideos: TrendSeries{
				X:      dailyLabels,
				Values: videoValues,
				Rates:  s.rates.CalcRates(videoValues),
			},
			NewUsers: TrendSeries{
				X:      dailyLabels,
				Values: userValues,
				Rates:  s.rates.CalcRates(userValues),
			},
		},
		Monthly: AdminMonthlyStats{
			NewVideos: TrendSeries{
				X:      monthlyLabels,
				Values: monthlyVideoValues,
				Rates:  s.rates.CalcRates(monthlyVideoValues),
			},
		},
	}

	return stats, nil
}
