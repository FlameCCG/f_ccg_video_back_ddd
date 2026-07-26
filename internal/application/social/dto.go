// Package social 是社交关系上下文的应用层：关注/取关用例、关系与列表查询、
// DTO（含读模型）与查询/协作端口。
//
// 本包不依赖 gin / gorm / redis：数据访问一律经端口，实现在 infrastructure。
package social

// SocialListUser 关注/粉丝列表条目（读模型）。
//
// 对外 JSON 由 interfaces 层的响应结构体（FocusUserInfo / FansUserInfo）承担，
// 本类型只承载查询结果字段，与原 user_service.SocialListUser 一一对应。
type SocialListUser struct {
	ID          uint
	Username    string
	Avatar      string
	Description string
	Level       int
}

// SocialListParams 关注/粉丝列表查询参数。
//
// OwnerID / ViewerID 用 int：原实现由 strconv.Atoi 解析 query 与 uid，
// 可能得到负数或 0，隐私判定与「是否本人」比较都在 int 域内完成，此处保持一致。
type SocialListParams struct {
	// OwnerID 被查看的用户（?id=）。
	OwnerID int
	// ViewerID 当前登录用户；未登录/解析失败为 0。
	ViewerID int
	Page     int
	PageSize int
	Keyword  string
}
