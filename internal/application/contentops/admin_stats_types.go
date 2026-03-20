package contentops

// AdminOverview 管理端站点总览（读模型）.
type AdminOverview struct {
	TotalUsers      int64 `json:"totalUsers"`
	OnlineUsers     int64 `json:"onlineUsers"`
	TotalVideos     int64 `json:"totalVideos"`
	TotalComments   int64 `json:"totalComments"`
	TodayLoginUsers int64 `json:"todayLoginUsers"`
	TodayRegisters  int64 `json:"todayRegisters"`
}

// TrendSeries 趋势序列（读模型）：横轴标签、数值与环比增长率.
type TrendSeries struct {
	X      []string  `json:"x"`
	Values []int64   `json:"values"`
	Rates  []float64 `json:"rates"`
}

// AdminDailyStats 管理端日维度统计（读模型）.
//
// LoginUsers 与 VisitUsers 不是同一个量：VisitUsers 取自站点打点的日 UV
// （含未登录访客），LoginUsers 是当日去重的登录用户数，与 AdminOverview.TodayLoginUsers
// 同口径 —— 管理端「今日登录」卡片的环比必须用它，用 UV 会给出另一个数。
type AdminDailyStats struct {
	VisitUsers    TrendSeries `json:"visitUsers"`
	LoginUsers    TrendSeries `json:"loginUsers"`
	PublishVideos TrendSeries `json:"publishVideos"`
	NewUsers      TrendSeries `json:"newUsers"`
}

// AdminMonthlyStats 管理端月维度统计（读模型）.
type AdminMonthlyStats struct {
	NewVideos TrendSeries `json:"newVideos"`
}

// AdminSiteStats 管理端站点统计（读模型）.
type AdminSiteStats struct {
	Overview AdminOverview     `json:"overview"`
	Daily    AdminDailyStats   `json:"daily"`
	Monthly  AdminMonthlyStats `json:"monthly"`
}
