package rbac

import (
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

// RouteLister 返回 HTTP 引擎注册的 method+path 清单。
//
// 端口由应用层声明，适配器位于 interfaces，由 bootstrap 绑定，保持 interfaces → application 的依赖方向。
type RouteLister interface {
	// Routes 返回引擎注册的全部路由。
	Routes() []rbacdomain.RouteInfo
}

// ResourceMetaLoader OpenAPI 资源说明加载端口（文件 IO + YAML 解析属基础设施关注点）。
type ResourceMetaLoader interface {
	// Load 读取并解析 OpenAPI 文档，返回以「METHOD path」为键的说明表。
	// 失败时返回空表与错误，调用方降级为默认说明。
	Load() (map[string]rbacdomain.ResourceMeta, error)
}
