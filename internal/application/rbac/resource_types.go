package rbac

// APIResourceItem 可用于权限配置的接口资源项。
type APIResourceItem struct {
	Path    string   `json:"path"`
	Method  string   `json:"method"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags,omitempty"`
}

// ResourceSyncResult 接口资源同步结果。
type ResourceSyncResult struct {
	Total   int
	Created int
	Updated int
	Deleted int
}
