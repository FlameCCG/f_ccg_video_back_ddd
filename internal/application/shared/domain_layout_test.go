package shared_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestDomainLayoutAndRoleDependencies 要求每个业务上下文只有一个领域包。
// 包内按业务能力分文件。shared 只保留既有枚举和事务契约，不扩大共享内核。
func TestDomainLayoutAndRoleDependencies(t *testing.T) {
	root := filepath.Join("..", "..", "domain")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		parts := strings.Split(rel, "/")
		if parts[0] == "shared" {
			if entry.IsDir() {
				if len(parts) == 1 {
					return nil
				}
				if len(parts) == 2 && (parts[1] == "enum" || parts[1] == "tx") {
					return nil
				}
				t.Errorf("%s: shared kernel only keeps enum and tx", rel)
				return fs.SkipDir
			}
			if len(parts) != 3 || (parts[1] != "enum" && parts[1] != "tx") || !strings.HasSuffix(parts[2], ".go") {
				t.Errorf("%s: shared kernel only keeps enum and tx", rel)
			}
			return nil
		}
		if entry.IsDir() {
			if len(parts) == 1 {
				return nil
			}
			t.Errorf("%s: keep a business context in one package", rel)
			return fs.SkipDir
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if len(parts) != 2 {
			t.Errorf("%s: keep a business context in one package", rel)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		owner := parts[0]
		if file.Name.Name != owner && file.Name.Name != owner+"_test" {
			t.Errorf("%s: package %q must match context %q", rel, file.Name.Name, owner)
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if strings.Contains(value, "/internal/domain/"+owner+"/") {
				t.Errorf("%s: context package must not import a nested role package", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
