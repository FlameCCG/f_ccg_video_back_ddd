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

var domainRoleDependencies = map[string]map[string]bool{
	"domainerr":   {},
	"valueobject": {"domainerr": true},
	"event":       {"domainerr": true, "valueobject": true},
	"entity":      {"domainerr": true, "valueobject": true, "event": true},
	"repository":  {"domainerr": true, "valueobject": true, "event": true, "entity": true},
	"port":        {"domainerr": true, "valueobject": true, "event": true, "entity": true},
	"service":     {"domainerr": true, "valueobject": true, "event": true, "entity": true, "repository": true, "port": true},
}

// TestDomainLayoutAndRoleDependencies 保护统一的目录职责及单向依赖。
// 模块只建立实际需要的角色包；shared 是显式共享内核，不套用业务模块布局。
func TestDomainLayoutAndRoleDependencies(t *testing.T) {
	root := filepath.Join("..", "..", "domain")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if parts[0] == "shared" {
			return nil
		}
		if len(parts) < 3 {
			t.Errorf("%s: put domain code in its responsibility package", rel)
			return nil
		}
		owner, role := parts[0], parts[1]
		allowed, known := domainRoleDependencies[role]
		if !known {
			t.Errorf("%s: unknown domain role %q", rel, role)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		if file.Name.Name != role && file.Name.Name != role+"_test" {
			t.Errorf("%s: package %q must match responsibility %q", rel, file.Name.Name, role)
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			prefix := modulePath + "/internal/domain/" + owner + "/"
			if !strings.HasPrefix(value, prefix) {
				continue
			}
			dependency := strings.Split(strings.TrimPrefix(value, prefix), "/")[0]
			if dependency != role && !allowed[dependency] {
				t.Errorf("%s: %s cannot depend on %s; keep model behavior independent of orchestration and storage", rel, role, dependency)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
