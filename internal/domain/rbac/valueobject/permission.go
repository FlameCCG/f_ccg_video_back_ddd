package valueobject

// PermissionItem 权限项（资源路径 + 操作方法）。
//
// 领域内部使用的纯值对象；HTTP 请求体里的同名结构体带 binding/label tag，保留在 handler 包。
type PermissionItem struct {
	Resource string
	Action   string
}
