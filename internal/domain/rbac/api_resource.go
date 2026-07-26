package rbac

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// uploadsPathPrefix 静态上传目录前缀，同步资源表时整体跳过。
const uploadsPathPrefix = "/uploads"

// ApiResource 接口资源实体（可用于权限配置的 method+path 清单）。
//
// Tags 保存原始 JSON 字节（与 PO 的 datatypes.JSON 等价），
// 使「标签是否变化」的比较逻辑与原实现逐字节一致，同时让领域层不依赖 gorm.io/datatypes。
type ApiResource struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Path      string
	Method    string
	Summary   string
	Tags      []byte
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
func IsResourceMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return true
	default:
		return false
	}
}

// BuildApiResources 把引擎路由清单转换为待落库的资源列表。
//
// 语义与原 service/rbac_service/resource_sync.go 一致：
// 过滤非资源方法与 /uploads 静态目录、按 method+path 去重、
// summary 缺省为「METHOD path」、tags 序列化失败时留空，最后按 path→method 排序。
func BuildApiResources(routes []RouteInfo, meta map[string]ResourceMeta) []*ApiResource {
	items := make([]*ApiResource, 0, len(routes))
	seen := make(map[string]struct{})

	for _, r := range routes {
		if !IsResourceMethod(r.Method) {
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

		m := LookupResourceMeta(meta, r.Method, r.Path)

		summary := strings.TrimSpace(m.Summary)
		if summary == "" {
			summary = fmt.Sprintf("%s %s", r.Method, r.Path)
		}

		var tagsJSON []byte

		if len(m.Tags) > 0 {
			if b, err := json.Marshal(m.Tags); err == nil {
				tagsJSON = b
			}
		}

		items = append(items, &ApiResource{
			Path:    r.Path,
			Method:  r.Method,
			Summary: summary,
			Tags:    tagsJSON,
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

// LookupResourceMeta 在 OpenAPI 元数据中按多种候选路径写法查找说明。
//
// 候选顺序：原始路径 → :param 转 {param} → 去掉 /v1 前缀 → 去前缀后再转 {param}。
func LookupResourceMeta(meta map[string]ResourceMeta, method, path string) ResourceMeta {
	method = strings.ToUpper(method)

	if meta == nil {
		return ResourceMeta{}
	}

	candidates := []string{
		path,
		normalizeParamPath(path),
	}

	if after, ok := strings.CutPrefix(path, "/v1"); ok {
		trimmed := after
		if trimmed != "" {
			candidates = append(candidates, trimmed, normalizeParamPath(trimmed))
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
func normalizeParamPath(path string) string {
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

// JSONEqual 语义比较两段 JSON 字节；任一侧不是合法 JSON 时退化为字节串比较。
func JSONEqual(a, b []byte) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	var (
		va any
		vb any
	)

	if err := json.Unmarshal(a, &va); err != nil {
		return string(a) == string(b)
	}

	if err := json.Unmarshal(b, &vb); err != nil {
		return string(a) == string(b)
	}

	return reflect.DeepEqual(va, vb)
}

// ResourceKey 资源表的去重键（方法大写 + 空格 + 路径）。
func ResourceKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// DecodeResourceTags 解析资源标签 JSON；空值或非法 JSON 时返回 nil。
func DecodeResourceTags(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}

	var tags []string
	_ = json.Unmarshal(raw, &tags)

	return tags
}
