package contentops

// AdminServerCORS 管理端可见的 server 配置读模型，仅暴露 corsOrigins，避免修改监听地址等进程级字段。
type AdminServerCORS struct {
	CorsOrigins []string `json:"corsOrigins"`
}
