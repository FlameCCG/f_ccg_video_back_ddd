package rbac

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// uploadsPathPrefix 静态上传目录前缀，同步资源表时整体跳过。
const uploadsPathPrefix = "/uploads"

// ApiResource 接口资源实体（可用于权限配置的 method+path 清单）。
//
// Tags 只表达业务标签；存储编码与历史格式兼容由持久化适配器处理。
type ApiResource struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Path      string
	Method    string
	Summary   string
	Tags      []string
}

// RouteInfo 引擎已注册的一条路由（method + path）。
type RouteInfo struct {
	Method string
	Path   string
}

// ResourceMeta OpenAPI 文档中某个 method+path 的补充说明。
type ResourceMeta struct {
	Summary string
	Tags    []string
}

// IsResourceMethod 判断 HTTP 方法是否需要纳入资源表。
func (ApiResource) IsResourceMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}

// BuildApiResources 把引擎路由清单转换为待落库的资源列表。
//
// 语义过滤非资源方法与 /uploads 静态目录、按 method+path 去重、
// summary 缺省为「METHOD path」，最后按 path→method 排序。
func (ApiResource) Build(routes []RouteInfo, meta map[string]ResourceMeta) []*ApiResource {
	items := make([]*ApiResource, 0, len(routes))
	seen := make(map[string]struct{})

	for _, r := range routes {
		if !(ApiResource{}).IsResourceMethod(r.Method) {
			continue
		}

		if strings.HasPrefix(r.Path, uploadsPathPrefix) {
			continue
		}

		key := r.Method + " " + r.Path
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		m := (ApiResource{}).LookupMeta(meta, r.Method, r.Path)

		summary := strings.TrimSpace(m.Summary)
		if summary == "" {
			summary = fmt.Sprintf("%s %s", r.Method, r.Path)
		}

		items = append(items, &ApiResource{
			Path:    r.Path,
			Method:  r.Method,
			Summary: summary,
			Tags:    slices.Clone(m.Tags),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Method < items[j].Method
		}

		return items[i].Path < items[j].Path
	})

	return items
}

// LookupMeta 在 OpenAPI 元数据中按多种候选路径写法查找说明。
//
// 候选顺序：原始路径 → :param 转 {param} → 去掉 /v1 前缀 → 去前缀后再转 {param}。
func (ApiResource) LookupMeta(meta map[string]ResourceMeta, method, path string) ResourceMeta {
	method = strings.ToUpper(method)

	if meta == nil {
		return ResourceMeta{}
	}

	candidates := []string{
		path,
		(ApiResource{}).normalizeParamPath(path),
	}

	if after, ok := strings.CutPrefix(path, "/v1"); ok {
		trimmed := after
		if trimmed != "" {
			candidates = append(candidates, trimmed, (ApiResource{}).normalizeParamPath(trimmed))
		}
	}

	for _, p := range candidates {
		if p == "" {
			continue
		}

		if m, ok := meta[method+" "+p]; ok {
			return m
		}
	}

	return ResourceMeta{}
}

// normalizeParamPath 把 gin 的 :param 路径段改写成 OpenAPI 的 {param} 写法。
func (ApiResource) normalizeParamPath(path string) string {
	if path == "" || !strings.Contains(path, "/:") {
		return path
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}

	return strings.Join(parts, "/")
}

// Key 是资源表的去重键（方法大写 + 空格 + 路径）。
func (ApiResource) Key(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// ResourceMetadataChange 是字段级元数据变更计划。标签始终表达本次目标集合。
type ResourceMetadataChange struct {
	Summary *string
	Tags    []string
}

// PlanMetadata 保留说明的空白比较规则。标签的存储差异由适配器裁定。
func (r ApiResource) PlanMetadata(target *ApiResource) ResourceMetadataChange {
	change := ResourceMetadataChange{Tags: slices.Clone(target.Tags)}
	if strings.TrimSpace(r.Summary) != strings.TrimSpace(target.Summary) {
		summary := target.Summary
		change.Summary = &summary
	}
	return change
}
