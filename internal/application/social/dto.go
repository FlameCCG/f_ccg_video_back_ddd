// Package social 编排关注、取关及关系列表查询。
//
// focus_service.go 负责关系写侧与粉丝指标更新，relation_service.go 负责关系与列表读取。
// 列表能否查看由 domain/social 的隐私规则判断，用户存在性和关注事实由端口取得。
// 关注写入后更新粉丝指标，用户搜索文档由 River 异步派生，阅读错误分支时要区分这些完成点。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package social

// ListUser 关注/粉丝列表条目（读模型）。
//
// 对外 JSON 由 interfaces 层的响应结构体（FocusUserInfo / FansUserInfo）承担，
// 本类型只承载查询结果字段，与原 user_service.SocialListUser 一一对应。
type ListUser struct {
	ID          uint
	Username    string
	Avatar      string
	Description string
	Level       int
}

// ListParams 关注/粉丝列表查询参数。
//
// OwnerID / ViewerID 用 int：原实现由 strconv.Atoi 解析 query 与 uid，
// 可能得到负数或 0，隐私判定与「是否本人」比较都在 int 域内完成，此处保持一致。
type ListParams struct {
	// OwnerID 被查看的用户（?id=）。
	OwnerID int
	// ViewerID 当前登录用户；未登录/解析失败为 0。
	ViewerID int
	Page     int
	PageSize int
	Keyword  string
}
