package social

// SocialListUser 关注或粉丝列表条目的读模型。
type SocialListUser struct {
	ID          uint
	Username    string
	Avatar      string
	Description string
	Level       int
}

// SocialListParams 关注或粉丝列表查询参数，OwnerID / ViewerID 使用 int，接收 query 与 uid 解析后的用户 ID。
type SocialListParams struct {
	// OwnerID 被查看的用户（?id=）。
	OwnerID int
	// ViewerID 当前登录用户；未登录/解析失败为 0。
	ViewerID int
	Page     int
	PageSize int
	Keyword  string
}
