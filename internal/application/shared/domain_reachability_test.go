package shared_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestDomainPackagesReachableFromEntrypoints 防止再次加入没有任何业务消费者的领域包。
// 覆盖两个 main 入口及 Wire 配置；测试自身的 import 不能让闲置领域包变成可达。
// 该检查只保护包级可达性，函数和声明仍需配合 deadcode / Staticcheck 审查。
func TestDomainPackagesReachableFromEntrypoints(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dependencies := make(map[string]map[string]bool)
	domainPackages := make(map[string]bool)
	var entrypoints []string
	for _, directory := range []string{"cmd", "internal", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			owner := modulePath + "/" + filepath.ToSlash(rel)
			if dependencies[owner] == nil {
				dependencies[owner] = make(map[string]bool)
			}
			if file.Name.Name == "main" && strings.HasPrefix(filepath.ToSlash(rel), "cmd/") {
				entrypoints = append(entrypoints, owner)
			}
			if strings.HasPrefix(filepath.ToSlash(rel), "internal/domain/") {
				domainPackages[owner] = true
			}
			for _, spec := range file.Imports {
				value, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				if matchesPackage(value, modulePath) {
					dependencies[owner][value] = true
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(entrypoints) == 0 {
		t.Fatal("no executable entrypoints found")
	}
	reachable := make(map[string]bool)
	for len(entrypoints) > 0 {
		current := entrypoints[0]
		entrypoints = entrypoints[1:]
		if reachable[current] {
			continue
		}
		reachable[current] = true
		for dependency := range dependencies[current] {
			entrypoints = append(entrypoints, dependency)
		}
	}
	var unused []string
	for name := range domainPackages {
		if !reachable[name] {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	for _, name := range unused {
		t.Errorf("domain package %s has no production or Wire consumer", name)
	}
}
