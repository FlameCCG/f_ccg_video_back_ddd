package shared_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 架构守护：检查生产源码（含带 build tag 的文件）的依赖方向。
// 测试可装配真实适配器，不受生产 import 约束；这些检查不代替领域规则测试。
const (
	modulePath          = "f_ccg_video_back_ddd"
	infraImportPrefix   = modulePath + "/internal/infrastructure"
	neutralConfigImport = modulePath + "/internal/config"
)

// collectImports 解析 dir 下全部非测试 .go 文件的 import，返回 文件 → 非法 import 列表。
func collectImports(t *testing.T, dir, bannedPrefix string, extraBanned ...string) map[string][]string {
	t.Helper()

	violations := make(map[string][]string)

	// 测试自身位于 dir 的子包（shared），向上定位目标层目录。
	root, err := filepath.Abs(dir)
	require.NoError(t, err, "resolve %s: %v", dir, err)

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()

		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range file.Imports {
			value, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}

			banned := matchesPackage(value, bannedPrefix)
			for _, extra := range extraBanned {
				banned = banned || matchesPackage(value, extra)
			}

			if banned {
				rel, _ := filepath.Rel(root, path)
				violations[rel] = append(violations[rel], value)
			}
		}

		return nil
	})
	require.NoError(t, err, "walk %s: %v", root, err)

	return violations
}

func reportViolations(t *testing.T, layer string, violations map[string][]string) {
	t.Helper()

	for file, imports := range violations {
		t.Errorf("%s 层违规：%s import %s", layer, file, strings.Join(imports, ", "))
	}
}

// TestApplicationDoesNotImportInfrastructure 禁止反向分层依赖及直接引入持久化、
// 消息、验证码等适配器库；Wire ProviderSet 与现有日志/AI 消息契约不在禁用范围。
func TestApplicationDoesNotImportInfrastructure(t *testing.T) {
	t.Parallel()

	reportViolations(t, "application", collectImports(t, "..", infraImportPrefix,
		modulePath+"/internal/interfaces", modulePath+"/internal/bootstrap", modulePath+"/conf",
		"github.com/gin-gonic/gin", "gorm.io", "github.com/redis/go-redis",
		"github.com/IBM/sarama", "github.com/minio/minio-go", "github.com/spf13/viper",
		"github.com/casbin", "github.com/mojocn/base64Captcha"))
}

// TestDomainDoesNotImportConfigOrInfrastructure 检查领域层禁止的框架与上层依赖。
func TestDomainDoesNotImportConfigOrInfrastructure(t *testing.T) {
	t.Parallel()

	reportViolations(t, "domain", collectImports(t, "../../domain", infraImportPrefix, neutralConfigImport,
		modulePath+"/internal/application", modulePath+"/internal/interfaces", modulePath+"/internal/bootstrap", modulePath+"/conf",
		"github.com/gin-gonic/gin", "gorm.io", "github.com/redis/go-redis",
		"github.com/IBM/sarama", "github.com/minio/minio-go", "github.com/spf13/viper",
		"github.com/casbin", "go.uber.org/zap"))
}

// net/http 的 Header/Response 仍用于现有 AI 流端口；创建 HTTP 请求和客户端
// 属于适配器实现，不能因为它是标准库就放在用例层。识别别名和点导入。
func TestApplicationDoesNotImplementHTTPClients(t *testing.T) {
	t.Parallel()

	banned := map[string]bool{
		"Client": true, "Transport": true, "DefaultClient": true, "DefaultTransport": true,
		"NewRequest": true, "NewRequestWithContext": true, "Get": true, "Post": true, "PostForm": true,
	}

	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}

		return inspectApplicationHTTPImports(t, path, file, banned)
	})
	require.NoError(t, err)
}

func TestImportGuardIncludesTaggedFilesAndRespectsPackageBoundaries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	for name, source := range map[string]string{
		"tagged.go":           "//go:build future\n\npackage sample\nimport infra `" + infraImportPrefix + "/persistence`\n",
		"permitted.go":        "package sample\nimport \"" + infraImportPrefix + "x\"\n",
		"integration_test.go": "package sample\nimport \"" + infraImportPrefix + "/persistence\"\n",
	} {
		{
			err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600)
			require.NoError(t, err)
		}
	}

	violations := collectImports(t, root, infraImportPrefix)
	require.Len(t, violations, 1, "violations=%v", violations)
	require.Len(t, violations["tagged.go"], 1, "violations=%v", violations)
}

func matchesPackage(value, prefix string) bool {
	return value == prefix || strings.HasPrefix(value, prefix+"/")
}

// 规则和用例不直接导入其它限界上下文的模型/服务；共享内核 shared 除外。
// 翻译与装配放在 bootstrap/adapters，具体能力由消费方端口声明。
func TestBoundedContextImports(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, layer := range []string{"domain", "application"} {
		t.Run(layer, func(t *testing.T) {
			root := filepath.Join("../..", layer)

			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}

				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}

				owner, _, _ := strings.Cut(filepath.ToSlash(rel), "/")

				file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
				if err != nil {
					return err
				}

				return inspectContextImports(t, layer, rel, owner, file)
			})
			require.NoError(t, err)
		})
	}
}

func inspectApplicationHTTPImports(t *testing.T, path string, file *ast.File, banned map[string]bool) error {
	t.Helper()

	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}

		if value != "net/http" {
			continue
		}

		alias := "http"
		if imp.Name != nil {
			alias = imp.Name.Name
		}

		if alias == "." {
			t.Errorf("%s: net/http dot import hides adapter usage", path)
			continue
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkg, ok := selector.X.(*ast.Ident)
			assert.False(t, ok && pkg.Name == alias && banned[selector.Sel.Name], "%s: HTTP implementation %s.%s belongs in infrastructure", path, alias, selector.Sel.Name)

			return true
		})
	}

	return nil
}

func inspectContextImports(t *testing.T, layer, rel, owner string, file *ast.File) error {
	t.Helper()

	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}

		parts := strings.Split(strings.TrimPrefix(value, modulePath+"/internal/"), "/")
		if !strings.HasPrefix(value, modulePath+"/internal/") || len(parts) < 2 {
			continue
		}

		assert.False(t, (parts[0] == "domain" || parts[0] == "application") && parts[1] != owner && parts[1] != "shared", "%s/%s imports foreign context %s; use a consumer-owned port", layer, rel, value)
	}

	return nil
}
