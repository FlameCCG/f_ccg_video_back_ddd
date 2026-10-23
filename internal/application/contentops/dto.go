// Package contentops 编排站点配置、轮播图与站点统计。
//
// site_config_service.go 经 shared.ConfigStore 修改配置，banner_service.go 组合规则、仓储与缓存。
// 配置发布到内存与写回 YAML 不是原子事务；文件写入失败时应区分当前生效值与重启后会加载的值。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package contentops

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// VisibleBannerParams 前台可见轮播图查询参数（原 api/banner_api.BannerListReq 的应用层入参）。
type VisibleBannerParams struct {
	// Type 轮播图类型原始值；0 表示未指定。
	Type int
	// PartitionID 分区 ID；nil 表示未指定。
	PartitionID *uint
}

// AdminBannerListParams 后台轮播图列表查询参数。
type AdminBannerListParams struct {
	// Page 页码。
	Page int
	// PageSize 每页条数。
	PageSize int
	// Type 轮播图类型原始值；0 表示未指定。
	Type int
	// PartitionID 分区 ID；nil 表示不限分区。
	PartitionID *uint
	// Show 显示状态；nil 表示不限显示状态。
	Show *bool
}

// CreateBannerParams 创建轮播图参数。
type CreateBannerParams struct {
	// Cover 图片链接。
	Cover string
	// Href 跳转链接。
	Href string
	// Show 是否显示；nil 时默认显示。
	Show *bool
	// Type 轮播图类型原始值；0 时默认首页轮播图。
	Type int
	// PartitionID 分区 ID；nil 视为 0。
	PartitionID *uint
}

// UpdateBannerParams 更新轮播图参数；指针字段为 nil 表示不更新。
type UpdateBannerParams struct {
	// ID 轮播图主键。
	ID uint
	// Cover 图片链接。
	Cover *string
	// Href 跳转链接。
	Href *string
	// Show 是否显示。
	Show *bool
	// Type 轮播图类型原始值。
	Type *int
	// PartitionID 分区 ID。
	PartitionID *uint
}

// TouchResult 站点打点结果（读模型，json tag 与原 site_service.TouchResult 一致）.
type TouchResult struct {
	PV     int64 `json:"pv"`
	UV     int64 `json:"uv"`
	Online int64 `json:"online"`
}

// HeartbeatResult 在线心跳结果（读模型，json tag 与原 site_service.HeartbeatResult 一致）.
type HeartbeatResult struct {
	UV     int64 `json:"uv"`
	Online int64 `json:"online"`
}

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

// AdminServerCORS 管理端可见的 server 配置（读模型）。
//
// 管理端仅暴露 corsOrigins，避免误改监听地址等进程级字段；
// json tag 与原 api/site_api.AdminServerCORSConfig 一致。
type AdminServerCORS struct {
	CorsOrigins []string `json:"corsOrigins"`
}

// bannerTypeOrZero 把请求中的类型原始值转为枚举；非正值表示未指定。
func bannerTypeOrZero(raw int) enum.BannerType {
	if raw > 0 {
		return enum.BannerType(raw)
	}

	return 0
}
