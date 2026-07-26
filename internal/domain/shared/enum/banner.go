// Package enum provides package functionality.
package enum

// BannerType 类型.
type BannerType int

// BannerTypeUser 及相关导出常量.
const (
	// BannerTypeHome 常量.
	BannerTypeHome BannerType = 1 // 首页/分区轮播图，partition_id=0 代表首页
	// BannerTypeTop 常量.
	BannerTypeTop BannerType = 2 // 顶部横幅
	// BannerTypeUser 常量.
	BannerTypeUser BannerType = 3 // 用户主页横幅
)
