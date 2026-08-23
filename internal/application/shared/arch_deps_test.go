package shared_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// 架构守护（见 docs/decisions.md D-020）：application 层不得 import 任何
// internal/infrastructure/**，配置访问只允许经本包（shared）的端口与
// 共享内核 internal/config 的数据类型；domain 层 additionally 禁止 import
// internal/config，保持零配置依赖。
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
	if err != nil {
		t.Fatalf("resolve %s: %v", dir, err)
	}

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
			value := strings.Trim(imp.Path.Value, `"`)

			banned := strings.HasPrefix(value, bannedPrefix)
			for _, extra := range extraBanned {
				banned = banned || value == extra || strings.HasPrefix(value, extra+"/")
			}

			if banned {
				rel, _ := filepath.Rel(root, path)
				violations[rel] = append(violations[rel], value)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return violations
}

func reportViolations(t *testing.T, layer string, violations map[string][]string) {
	t.Helper()

	for file, imports := range violations {
		t.Errorf("%s 层违规：%s import %s", layer, file, strings.Join(imports, ", "))
	}
}

// TestApplicationDoesNotImportInfrastructure 保证 application 用例层零基础设施依赖：
// 数据访问、外部服务、配置机制一律经端口（domain/** 或 application/shared）。
func TestApplicationDoesNotImportInfrastructure(t *testing.T) {
	t.Parallel()

	reportViolations(t, "application", collectImports(t, "..", infraImportPrefix))
}

// TestDomainDoesNotImportConfigOrInfrastructure 保证领域层零框架与零配置依赖（D-013/D-020 规则不放松）。
func TestDomainDoesNotImportConfigOrInfrastructure(t *testing.T) {
	t.Parallel()

	reportViolations(t, "domain", collectImports(t, "../../domain", infraImportPrefix, neutralConfigImport))
}
