package shared_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// 全实体 Save 会把与本次动作无关的余额、状态、计数及媒体字段一起覆盖。
func TestPersistenceDoesNotSaveWholeAggregates(t *testing.T) {
	err := filepath.WalkDir("../../infrastructure/persistence", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Save" {
					t.Errorf("%s: persist only the fields belonging to the business action", path)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// CLI 的基础设施包只做迁移和外部引擎适配，业务写入经 bootstrap 进入所属上下文。
func TestProvisioningInfrastructureDoesNotWriteDomainState(t *testing.T) {
	err := filepath.WalkDir("../../infrastructure/persistence/provisioning", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		banned := map[string]bool{"Save": true, "Create": true, "Updates": true, "Update": true, "UpdateColumn": true, "UpdateColumns": true, "Delete": true, "Association": true}
		ast.Inspect(file, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && banned[sel.Sel.Name] {
					t.Errorf("%s: %s bypasses context-owned domain actions", path, sel.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
