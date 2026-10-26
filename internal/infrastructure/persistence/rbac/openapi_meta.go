package rbac

import (
	"f_ccg_video_back_ddd/pkg/apperr"
	"os"
	"path/filepath"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/rbac"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"gopkg.in/yaml.v3"
)

// OpenAPIMetaLoader 从 docs/openapi.yaml 读取接口说明与标签。
type OpenAPIMetaLoader struct{}

// 编译期断言：满足应用层声明的端口。
var _ app.ResourceMetaLoader = (*OpenAPIMetaLoader)(nil)

// NewOpenAPIMetaLoader 构造 OpenAPI 说明加载器。
func NewOpenAPIMetaLoader() *OpenAPIMetaLoader {
	return &OpenAPIMetaLoader{}
}

// Load 读取并解析 OpenAPI 文档，返回以「METHOD path」为键的说明表。
//
// 与原实现一致：任何失败都返回空表 + 错误，由调用方降级为默认说明。
func (l *OpenAPIMetaLoader) Load() (map[string]domain.ResourceMeta, error) {
	path, err := resolveOpenAPIPath()
	if err != nil {
		return map[string]domain.ResourceMeta{}, err
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- The resolver selects only fixed docs/openapi.yaml locations under the installation.
	if err != nil {
		return map[string]domain.ResourceMeta{}, apperr.Wrap("infrastructure/persistence/rbac/openapi_meta", domain.ErrOpenAPIRead, err)
	}

	var doc struct {
		Paths map[string]map[string]struct {
			Summary string   `yaml:"summary"`
			Tags    []string `yaml:"tags"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return map[string]domain.ResourceMeta{}, apperr.Wrap("infrastructure/persistence/rbac/openapi_meta", domain.ErrOpenAPIParse, err)
	}

	result := make(map[string]domain.ResourceMeta)

	for p, ops := range doc.Paths {
		for method, op := range ops {
			key := strings.ToUpper(method) + " " + p
			result[key] = domain.ResourceMeta{Summary: op.Summary, Tags: op.Tags}
		}
	}

	return result, nil
}

// resolveOpenAPIPath 依次尝试工作目录与可执行文件同级目录下的 docs/openapi.yaml。
func resolveOpenAPIPath() (string, error) {
	candidates := []string{
		"docs/openapi.yaml",
		"./docs/openapi.yaml",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	if exe, err := os.Executable(); err == nil {
		base := filepath.Dir(exe)

		candidate := filepath.Join(base, "docs", "openapi.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", domain.ErrOpenAPINotFound
}
