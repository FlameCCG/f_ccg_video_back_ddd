package dynamic

// AdminDynamicListParams 管理端动态列表查询参数.
type AdminDynamicListParams struct {
	UserID   uint
	Username string
	Keyword  string
	Page     int
	PageSize int
}
