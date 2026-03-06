// Package enum 保存多个上下文共同使用的枚举与持久化取值。
//
// 这些整数或字符串既可能进入数据库，也可能出现在 JSON、缓存和消息中，是兼容性契约。
// 不要为了调整展示顺序重排 iota 常量，也不要仅为改文案就修改已持久化的字符串值。
// 新增状态应同时检查领域转换规则、查询过滤和接口响应；仅新增常量不会自动接入业务流程。
// 状态的含义在本包说明，是否允许状态转换仍由所属上下文的领域规则决定。
package enum

// BannerType 区分首页/分区轮播、顶部横幅与用户主页横幅，整数值直接参与落库和接口传输。
type BannerType int

const (
	// BannerTypeHome 表示首页或分区轮播图，具体位置结合 partition_id 判断。
	BannerTypeHome BannerType = 1 // 首页/分区轮播图，partition_id=0 代表首页
	// BannerTypeTop 表示站点顶部横幅。
	BannerTypeTop BannerType = 2 // 顶部横幅
	// BannerTypeUser 表示用户主页横幅。
	BannerTypeUser BannerType = 3 // 用户主页横幅
)
